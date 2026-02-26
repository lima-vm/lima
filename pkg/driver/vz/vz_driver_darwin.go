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
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/Code-Hex/vz/v3"
	"github.com/coreos/go-semver/semver"
	"github.com/docker/go-units"
	"github.com/lima-vm/go-qcow2reader/image"
	"github.com/lima-vm/go-qcow2reader/image/asif"
	"github.com/lima-vm/go-qcow2reader/image/raw"
	"github.com/sirupsen/logrus"

	"github.com/lima-vm/lima/v2/pkg/driver"
	"github.com/lima-vm/lima/v2/pkg/driverutil"
	"github.com/lima-vm/lima/v2/pkg/guestpatch/macos"
	"github.com/lima-vm/lima/v2/pkg/hostagent/events"
	"github.com/lima-vm/lima/v2/pkg/imgutil/nativeimgutil/asifutil"
	"github.com/lima-vm/lima/v2/pkg/limatype"
	"github.com/lima-vm/lima/v2/pkg/limatype/filenames"
	"github.com/lima-vm/lima/v2/pkg/limayaml"
	"github.com/lima-vm/lima/v2/pkg/osutil"
	"github.com/lima-vm/lima/v2/pkg/ptr"
	"github.com/lima-vm/lima/v2/pkg/reflectutil"
	"github.com/lima-vm/lima/v2/pkg/sshutil"
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

	SSHLocalPort    int
	vSockPort       int
	virtioPort      string
	rosettaEnabled  bool
	rosettaBinFmt   bool
	diskImageFormat image.Type

	machine                    *virtualMachineWrapper
	waitSSHLocalPortAccessible <-chan any

	onVsockEvent func(*events.VsockEvent)
}

var (
	_ driver.Driver            = (*LimaVzDriver)(nil)
	_ driver.VsockEventEmitter = (*LimaVzDriver)(nil)
)

// SetVsockEventCallback implements driver.VsockEventEmitter.
func (l *LimaVzDriver) SetVsockEventCallback(callback func(*events.VsockEvent)) {
	l.onVsockEvent = callback
}

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

	var vzOpts limatype.VZOpts
	if l.Instance.Config.VMOpts[limatype.VZ] != nil {
		if err := limayaml.Convert(l.Instance.Config.VMOpts[limatype.VZ], &vzOpts, "vmOpts.vz"); err != nil {
			logrus.WithError(err).Warnf("Couldn't convert %q", l.Instance.Config.VMOpts[limatype.VZ])
		}
	}

	if runtime.GOOS == "darwin" && limayaml.IsNativeArch(limatype.AARCH64) {
		if vzOpts.Rosetta.Enabled != nil {
			l.rosettaEnabled = *vzOpts.Rosetta.Enabled
		}
	}
	if vzOpts.Rosetta.BinFmt != nil {
		l.rosettaBinFmt = *vzOpts.Rosetta.BinFmt
	}
	if vzOpts.DiskImageFormat != nil {
		l.diskImageFormat = *vzOpts.DiskImageFormat
	} else {
		l.diskImageFormat = raw.Type
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

	if cfg.SSH.OverVsock == nil {
		cfg.SSH.OverVsock = ptr.Of(true)
	}

	var vzOpts limatype.VZOpts
	if err := limayaml.Convert(cfg.VMOpts[limatype.VZ], &vzOpts, "vmOpts.vz"); err != nil {
		logrus.WithError(err).Warnf("Couldn't convert %q", cfg.VMOpts[limatype.VZ])
	}

	//nolint:staticcheck // Migration of top-level Rosetta if specified
	if (vzOpts.Rosetta.Enabled == nil && vzOpts.Rosetta.BinFmt == nil) && (!isEmpty(cfg.Rosetta)) {
		logrus.Debug("Migrating top-level Rosetta configuration to vmOpts.vz.rosetta")
		vzOpts.Rosetta = cfg.Rosetta
	}
	//nolint:staticcheck // Warning about both top-level and vmOpts.vz.Rosetta being set
	if (vzOpts.Rosetta.Enabled != nil && vzOpts.Rosetta.BinFmt != nil) && (!isEmpty(cfg.Rosetta)) {
		logrus.Warn("Both top-level 'rosetta' and 'vmOpts.vz.rosetta' are configured. Using vmOpts.vz.rosetta. Top-level 'rosetta' is deprecated.")
	}

	if vzOpts.Rosetta.Enabled == nil {
		vzOpts.Rosetta.Enabled = ptr.Of(false)
	}
	if vzOpts.Rosetta.BinFmt == nil {
		vzOpts.Rosetta.BinFmt = ptr.Of(false)
	}
	if vzOpts.DiskImageFormat == nil {
		vzOpts.DiskImageFormat = ptr.Of(raw.Type)
	}

	var opts any
	if err := limayaml.Convert(vzOpts, &opts, ""); err != nil {
		logrus.WithError(err).Warnf("Couldn't convert %+v", vzOpts)
	}
	if cfg.VMOpts == nil {
		cfg.VMOpts = limatype.VMOpts{}
	}
	cfg.VMOpts[limatype.VZ] = opts

	return validateConfig(ctx, cfg)
}

func isEmpty(r limatype.Rosetta) bool {
	return r.Enabled == nil && r.BinFmt == nil
}

//go:embed boot.Linux/*.sh
var bootLinuxFS embed.FS

func (l *LimaVzDriver) BootScripts() (map[string][]byte, error) {
	scripts := make(map[string][]byte)

	entries, err := bootLinuxFS.ReadDir("boot.Linux")
	if err == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			entryPath := "boot.Linux/" + entry.Name()

			content, err := bootLinuxFS.ReadFile(entryPath)
			if err != nil {
				return nil, err
			}

			scripts[entryPath] = content
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
	var vzOpts limatype.VZOpts
	if err := limayaml.Convert(cfg.VMOpts[limatype.VZ], &vzOpts, "vmOpts.vz"); err != nil {
		logrus.WithError(err).Warnf("Couldn't convert %q", cfg.VMOpts[limatype.VZ])
	}
	switch *vzOpts.DiskImageFormat {
	case raw.Type:
	case asif.Type:
		if macOSProductVersion.LessThan(*semver.New("26.0.0")) {
			return fmt.Errorf("vmOpts.vz.diskImageFormat=%q requires macOS 26 or higher to run, got %q", asif.Type, macOSProductVersion)
		}
	default:
		return fmt.Errorf("field `vmOpts.vz.diskImageFormat` must be %q or %q, got %q", raw.Type, asif.Type, *vzOpts.DiskImageFormat)
	}
	return nil
}

func (l *LimaVzDriver) Create(_ context.Context) error {
	identifierFile := filepath.Join(l.Instance.Dir, filenames.VzIdentifier)
	if *l.Instance.Config.OS == limatype.DARWIN {
		_, err := getMacMachineIdentifier(identifierFile)
		return err
	}
	_, err := getGenericMachineIdentifier(identifierFile)
	return err
}

func (l *LimaVzDriver) CreateDisk(ctx context.Context) error {
	if *l.Instance.Config.OS == limatype.DARWIN {
		disk := filepath.Join(l.Instance.Dir, filenames.Disk)
		if !osutil.FileExists(disk) {
			if err := l.createDiskMacOSGuest(ctx); err != nil {
				return err
			}
		}

		patchedMarker := disk + ".patched" // empty file
		if !osutil.FileExists(patchedMarker) {
			logrus.Infof("Patching macOS disk %q", disk)
			if err := macos.Patch(ctx, disk); err != nil {
				return err
			}
			if err := os.WriteFile(patchedMarker, []byte{}, 0o644); err != nil {
				return err
			}
		}
	}
	return driverutil.EnsureDisk(ctx, l.Instance.Dir, *l.Instance.Config.Disk, l.diskImageFormat)
}

// createDiskMacOSGuest creates `disk` and installs macOS from `image` on it.
// The function must not be called if `disk` already exists.
//
// The function creates the following files:
// - `image.ipsw`: hardlink to `image` (".ipsw" suffix is required by VZMacOSInstaller)
// - `disk`: ASIF disk
//
// TODO: consider removing IPSW after successful installation.
func (l *LimaVzDriver) createDiskMacOSGuest(ctx context.Context) error {
	disk := filepath.Join(l.Instance.Dir, filenames.Disk)

	diskSize, err := units.RAMInBytes(*l.Instance.Config.Disk)
	if err != nil {
		return fmt.Errorf("invalid disk size %q: %w", *l.Instance.Config.Disk, err)
	}
	if err := asifutil.NewASIF(disk, diskSize); err != nil {
		return err
	}

	if err = ensureIPSW(l.Instance.Dir); err != nil {
		return err
	}
	ipsw := filepath.Join(l.Instance.Dir, filenames.ImageIPSW)

	vm, err := createVMForMacInstaller(ctx, l.Instance)
	if err != nil {
		return err
	}

	logrus.Info("Running macOS installer (takes a few minutes)")
	// FIXME: do we need to run the installer for every new instance,
	// or can we safely reuse the installed disk image?
	if err := installMacOS(ctx, vm, ipsw); err != nil {
		return fmt.Errorf("failed to install macOS: %w", err)
	}

	return nil
}

func (l *LimaVzDriver) Start(ctx context.Context) (chan error, error) {
	logrus.Infof("Starting VZ (hint: to watch the boot progress, see %q)", filepath.Join(l.Instance.Dir, "serial*.log"))
	vm, waitSSHLocalPortAccessible, errCh, err := startVM(ctx, l.Instance, l.SSHLocalPort, l.onVsockEvent)
	if err != nil {
		if errors.Is(err, vz.ErrUnsupportedOSVersion) {
			return nil, fmt.Errorf("vz driver requires macOS 13 or higher to run: %w", err)
		}
		return nil, err
	}
	l.machine = vm
	l.waitSSHLocalPortAccessible = waitSSHLocalPortAccessible

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

func (l *LimaVzDriver) requestStopViaSSH(ctx context.Context) error {
	sshExe, err := sshutil.NewSSHExe()
	if err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, sshExe.Exe,
		append(sshExe.Args, "-F", l.Instance.SSHConfigFile, l.Instance.Hostname, "--",
			"sudo", "/sbin/shutdown", "-h", "now")...)
	logrus.Infof("Running shutdown command in the VM: %v", cmd.Args)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to run %v: %w (output=%s)", cmd.Args, err, string(out))
	}
	return nil
}

func (l *LimaVzDriver) Stop(ctx context.Context) error {
	logrus.Info("Shutting down VZ")
	canStop := l.machine.CanRequestStop()

	if canStop {
		_, err := l.machine.RequestStop()
		if err != nil {
			return err
		}

		if *l.Instance.Config.OS == limatype.DARWIN {
			// macOS VM does not respond to l.machine.RequestStop(),
			// so we need to run `shutdown -h now` in the VM via SSH for graceful shutdown.
			if err := l.requestStopViaSSH(ctx); err != nil {
				logrus.WithError(err).Warn("Failed to request shutdown via SSH")
			}
		}

		// Most Linux machines shutdown within 5 seconds, but macOS machines can take longer.
		timeout := time.After(30 * time.Second)
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
		RosettaEnabled:       l.rosettaEnabled,
		RosettaBinFmt:        l.rosettaBinFmt,
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

func (l *LimaVzDriver) AdditionalSetupForSSH(_ context.Context) error {
	<-l.waitSSHLocalPortAccessible
	return nil
}
