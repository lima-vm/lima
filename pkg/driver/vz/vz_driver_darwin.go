//go:build darwin && !no_vz

// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package vz

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"net"
	"path/filepath"
	"runtime"
	"time"

	"github.com/Code-Hex/vz/v3"
	"github.com/coreos/go-semver/semver"
	"github.com/sirupsen/logrus"

	"github.com/lima-vm/lima/v2/pkg/driver"
	"github.com/lima-vm/lima/v2/pkg/limatype"
	"github.com/lima-vm/lima/v2/pkg/limayaml"
	"github.com/lima-vm/lima/v2/pkg/osutil"
	"github.com/lima-vm/lima/v2/pkg/ptr"
	"github.com/lima-vm/lima/v2/pkg/reflectutil"
)

var knownYamlProperties = []string{
	"AdditionalDisks",
	"Arch",
	"Audio",
	"CACertificates",
	"Containerd",
	"CopyToHost",
	"CPUs",
	"CPUType",
	"Disk",
	"DNS",
	"Env",
	"Firmware",
	"GuestInstallPrefix",
	"HostResolver",
	"Images",
	"Memory",
	"Message",
	"MinimumLimaVersion",
	"Mounts",
	"MountType",
	"MountTypesUnsupported",
	"MountInotify",
	"NestedVirtualization",
	"Networks",
	"OS",
	"Param",
	"Plain",
	"PortForwards",
	"Probes",
	"PropagateProxyEnv",
	"Provision",
	"Rosetta",
	"SSH",
	"TimeZone",
	"UpgradePackages",
	"User",
	"Video",
	"VMType",
	"VMOpts",
}

const Enabled = true

type LimaVzDriver struct {
	Instance *limatype.Instance

	SSHLocalPort int
	vSockPort    int
	virtioPort   string

	machine *virtualMachineWrapper
}

var _ driver.Driver = (*LimaVzDriver)(nil)

func New() *LimaVzDriver {
	return &LimaVzDriver{
		vSockPort:  2222,
		virtioPort: "",
	}
}

func (l *LimaVzDriver) Configure(inst *limatype.Instance) *driver.ConfiguredDriver {
	l.Instance = inst
	l.SSHLocalPort = inst.SSHLocalPort

	if l.Instance.Config.MountType != nil {
		mountTypesUnsupported := make(map[string]struct{})
		for _, f := range l.Instance.Config.MountTypesUnsupported {
			mountTypesUnsupported[f] = struct{}{}
		}

		if _, ok := mountTypesUnsupported[*l.Instance.Config.MountType]; ok {
			// We cannot return an error here, but Validate() will return it.
			logrus.Warnf("Unsupported mount type: %q", *l.Instance.Config.MountType)
		}
	}

	return &driver.ConfiguredDriver{
		Driver: l,
	}
}

func (l *LimaVzDriver) FillConfig(ctx context.Context, cfg *limatype.LimaYAML, _ string) error {
	if cfg.VMType == nil {
		cfg.VMType = ptr.Of(limatype.VZ)
	}

	if cfg.MountType == nil {
		cfg.MountType = ptr.Of(limatype.VIRTIOFS)
	}

	//nolint:staticcheck // Migration of top-level Rosetta if specified
	if (cfg.VMOpts.VZ.Rosetta.Enabled == nil && cfg.VMOpts.VZ.Rosetta.BinFmt == nil) && (!isEmpty(cfg.Rosetta)) {
		logrus.Debug("Migrating top-level Rosetta configuration to vmOpts.vz.rosetta")
		cfg.VMOpts.VZ.Rosetta = cfg.Rosetta
	}
	//nolint:staticcheck // Warning about both top-level and vmOpts.vz.Rosetta being set
	if (cfg.VMOpts.VZ.Rosetta.Enabled != nil && cfg.VMOpts.VZ.Rosetta.BinFmt != nil) && (!isEmpty(cfg.Rosetta)) {
		logrus.Warn("Both top-level 'rosetta' and 'vmOpts.vz.rosetta' are configured. Using vmOpts.vz.rosetta. Top-level 'rosetta' is deprecated.")
	}

	if cfg.VMOpts.VZ.Rosetta.Enabled == nil {
		cfg.VMOpts.VZ.Rosetta.Enabled = ptr.Of(false)
	}
	if cfg.VMOpts.VZ.Rosetta.BinFmt == nil {
		cfg.VMOpts.VZ.Rosetta.BinFmt = ptr.Of(false)
	}

	return validateConfig(ctx, cfg)
}

func isEmpty(r limatype.Rosetta) bool {
	return r.Enabled == nil && r.BinFmt == nil
}

//go:embed boot/*.sh
var bootFS embed.FS

func (l *LimaVzDriver) BootScripts() (map[string][]byte, error) {
	scripts := make(map[string][]byte)

	entries, err := bootFS.ReadDir("boot")
	if err == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}

			content, err := bootFS.ReadFile("boot/" + entry.Name())
			if err != nil {
				return nil, err
			}

			scripts[entry.Name()] = content
		}
	}

	return scripts, nil
}

func (l *LimaVzDriver) Validate(ctx context.Context) error {
	return validateConfig(ctx, l.Instance.Config)
}

func validateConfig(_ context.Context, cfg *limatype.LimaYAML) error {
	if cfg == nil {
		return errors.New("configuration is nil")
	}
	macOSProductVersion, err := osutil.ProductVersion()
	if err != nil {
		return err
	}
	if macOSProductVersion.LessThan(*semver.New("13.0.0")) {
		return errors.New("VZ driver requires macOS 13 or higher to run")
	}
	if runtime.GOARCH == "amd64" && macOSProductVersion.LessThan(*semver.New("15.5.0")) {
		logrus.Warnf("vmType %s: On Intel Mac, macOS 15.5 or later is required to run Linux 6.12 or later. "+
			"Update macOS, or change vmType to \"qemu\" if the VM does not start up. (https://github.com/lima-vm/lima/issues/3334)",
			*cfg.VMType)
	}
	if cfg.MountType != nil && *cfg.MountType == limatype.NINEP {
		return fmt.Errorf("field `mountType` must be %q or %q for VZ driver , got %q", limatype.REVSSHFS, limatype.VIRTIOFS, *cfg.MountType)
	}
	if *cfg.Firmware.LegacyBIOS {
		logrus.Warnf("vmType %s: ignoring `firmware.legacyBIOS`", *cfg.VMType)
	}
	for _, f := range cfg.Firmware.Images {
		switch f.VMType {
		case "", limatype.VZ:
			if f.Arch == *cfg.Arch {
				return errors.New("`firmware.images` configuration is not supported for VZ driver")
			}
		}
	}
	if unknown := reflectutil.UnknownNonEmptyFields(cfg, knownYamlProperties...); cfg.VMType != nil && len(unknown) > 0 {
		logrus.Warnf("vmType %s: ignoring %+v", *cfg.VMType, unknown)
	}

	if !limayaml.IsNativeArch(*cfg.Arch) {
		return fmt.Errorf("unsupported arch: %q", *cfg.Arch)
	}

	for i, image := range cfg.Images {
		if unknown := reflectutil.UnknownNonEmptyFields(image, "File", "Kernel", "Initrd"); len(unknown) > 0 {
			logrus.Warnf("vmType %s: ignoring images[%d]: %+v", *cfg.VMType, i, unknown)
		}
	}

	for i, mount := range cfg.Mounts {
		if unknown := reflectutil.UnknownNonEmptyFields(mount, "Location",
			"MountPoint",
			"Writable",
			"SSHFS",
			"NineP",
		); len(unknown) > 0 {
			logrus.Warnf("vmType %s: ignoring mounts[%d]: %+v", *cfg.VMType, i, unknown)
		}
	}

	for i, nw := range cfg.Networks {
		if unknown := reflectutil.UnknownNonEmptyFields(nw, "VZNAT",
			"Lima",
			"Socket",
			"MACAddress",
			"Metric",
			"Interface",
		); len(unknown) > 0 {
			logrus.Warnf("vmType %s: ignoring networks[%d]: %+v", *cfg.VMType, i, unknown)
		}

		field := fmt.Sprintf("networks[%d]", i)
		switch {
		case nw.Lima != "":
			if nw.VZNAT != nil && *nw.VZNAT {
				return fmt.Errorf("field `%s.lima` and field `%s.vzNAT` are mutually exclusive", field, field)
			}
		case nw.Socket != "":
			if nw.VZNAT != nil && *nw.VZNAT {
				return fmt.Errorf("field `%s.socket` and field `%s.vzNAT` are mutually exclusive", field, field)
			}
		case nw.VZNAT != nil && *nw.VZNAT:
			if nw.Lima != "" {
				return fmt.Errorf("field `%s.vzNAT` and field `%s.lima` are mutually exclusive", field, field)
			}
			if nw.Socket != "" {
				return fmt.Errorf("field `%s.vzNAT` and field `%s.socket` are mutually exclusive", field, field)
			}
		}
	}

	switch audioDevice := *cfg.Audio.Device; audioDevice {
	case "":
	case "vz", "default", "none":
	default:
		logrus.Warnf("field `audio.device` must be \"vz\", \"default\", or \"none\" for VZ driver, got %q", audioDevice)
	}

	switch videoDisplay := *cfg.Video.Display; videoDisplay {
	case "vz", "default", "none":
	default:
		logrus.Warnf("field `video.display` must be \"vz\", \"default\", or \"none\" for VZ driver , got %q", videoDisplay)
	}
	return nil
}

func (l *LimaVzDriver) Create(_ context.Context) error {
	_, err := getMachineIdentifier(l.Instance)
	return err
}

func (l *LimaVzDriver) CreateDisk(ctx context.Context) error {
	return EnsureDisk(ctx, l.Instance)
}

func (l *LimaVzDriver) Start(ctx context.Context) (chan error, error) {
	logrus.Infof("Starting VZ (hint: to watch the boot progress, see %q)", filepath.Join(l.Instance.Dir, "serial*.log"))
	vm, errCh, err := startVM(ctx, l.Instance, l.SSHLocalPort)
	if err != nil {
		if errors.Is(err, vz.ErrUnsupportedOSVersion) {
			return nil, fmt.Errorf("vz driver requires macOS 13 or higher to run: %w", err)
		}
		return nil, err
	}
	l.machine = vm

	return errCh, nil
}

func (l *LimaVzDriver) canRunGUI() bool {
	switch *l.Instance.Config.Video.Display {
	case "vz", "default":
		return true
	default:
		return false
	}
}

func (l *LimaVzDriver) RunGUI() error {
	if l.canRunGUI() {
		return l.machine.StartGraphicApplication(1920, 1200)
	}
	return fmt.Errorf("RunGUI is not supported for the given driver '%s' and display '%s'", "vz", *l.Instance.Config.Video.Display)
}

func (l *LimaVzDriver) Stop(_ context.Context) error {
	logrus.Info("Shutting down VZ")
	canStop := l.machine.CanRequestStop()

	if canStop {
		_, err := l.machine.RequestStop()
		if err != nil {
			return err
		}

		timeout := time.After(5 * time.Second)
		ticker := time.NewTicker(500 * time.Millisecond)
		for {
			select {
			case <-timeout:
				return errors.New("vz timeout while waiting for stop status")
			case <-ticker.C:
				l.machine.mu.Lock()
				stopped := l.machine.stopped
				l.machine.mu.Unlock()
				if stopped {
					return nil
				}
			}
		}
	}

	return errors.New("vz: CanRequestStop is not supported")
}

func (l *LimaVzDriver) GuestAgentConn(_ context.Context) (net.Conn, string, error) {
	for _, socket := range l.machine.SocketDevices() {
		connect, err := socket.Connect(uint32(l.vSockPort))
		return connect, "vsock", err
	}

	return nil, "", errors.New("unable to connect to guest agent via vsock port 2222")
}

func (l *LimaVzDriver) Info() driver.Info {
	var info driver.Info

	info.Name = "vz"
	info.VsockPort = l.vSockPort
	info.VirtioPort = l.virtioPort
	if l.Instance != nil {
		info.InstanceDir = l.Instance.Dir
	}

	var guiFlag bool
	if l.Instance != nil {
		guiFlag = l.canRunGUI()
	}
	info.Features = driver.DriverFeatures{
		DynamicSSHAddress:    false,
		SkipSocketForwarding: false,
		CanRunGUI:            guiFlag,
	}
	return info
}

func (l *LimaVzDriver) SSHAddress(_ context.Context) (string, error) {
	return "127.0.0.1", nil
}

func (l *LimaVzDriver) InspectStatus(_ context.Context, _ *limatype.Instance) string {
	return ""
}

func (l *LimaVzDriver) Delete(_ context.Context) error {
	return nil
}

func (l *LimaVzDriver) Register(_ context.Context) error {
	return nil
}

func (l *LimaVzDriver) Unregister(_ context.Context) error {
	return nil
}

func (l *LimaVzDriver) ChangeDisplayPassword(_ context.Context, _ string) error {
	return nil
}

func (l *LimaVzDriver) DisplayConnection(_ context.Context) (string, error) {
	return "", nil
}

func (l *LimaVzDriver) CreateSnapshot(_ context.Context, _ string) error {
	return errUnimplemented
}

func (l *LimaVzDriver) ApplySnapshot(_ context.Context, _ string) error {
	return errUnimplemented
}

func (l *LimaVzDriver) DeleteSnapshot(_ context.Context, _ string) error {
	return errUnimplemented
}

func (l *LimaVzDriver) ListSnapshots(_ context.Context) (string, error) {
	return "", errUnimplemented
}

func (l *LimaVzDriver) ForwardGuestAgent() bool {
	// If driver is not providing, use host agent
	return l.vSockPort == 0 && l.virtioPort == ""
}
