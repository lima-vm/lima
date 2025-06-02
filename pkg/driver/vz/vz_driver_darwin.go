//go:build darwin && !no_vz

// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package vz

import (
	"context"
	"errors"
	"fmt"
	"net"
	"path/filepath"
	"runtime"
	"time"

	"github.com/Code-Hex/vz/v3"
	"github.com/coreos/go-semver/semver"
	"github.com/sirupsen/logrus"

	"github.com/lima-vm/lima/pkg/driver"
	"github.com/lima-vm/lima/pkg/limayaml"
	"github.com/lima-vm/lima/pkg/osutil"
	"github.com/lima-vm/lima/pkg/reflectutil"
	"github.com/lima-vm/lima/pkg/store"
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
}

const Enabled = true

type LimaVzDriver struct {
	Instance *store.Instance

	SSHLocalPort int
	VSockPort    int
	VirtioPort   string

	machine *virtualMachineWrapper
}

var _ driver.Driver = (*LimaVzDriver)(nil)

func New() *LimaVzDriver {
	return &LimaVzDriver{
		VSockPort:  2222,
		VirtioPort: "",
	}
}

func (l *LimaVzDriver) SetConfig(inst *store.Instance, sshLocalPort int) {
	l.Instance = inst
	l.SSHLocalPort = sshLocalPort
}

func (l *LimaVzDriver) Validate() error {
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
			*l.Instance.Config.VMType)
	}
	if *l.Instance.Config.MountType == limayaml.NINEP {
		return fmt.Errorf("field `mountType` must be %q or %q for VZ driver , got %q", limayaml.REVSSHFS, limayaml.VIRTIOFS, *l.Instance.Config.MountType)
	}
	if *l.Instance.Config.Firmware.LegacyBIOS {
		logrus.Warnf("vmType %s: ignoring `firmware.legacyBIOS`", *l.Instance.Config.VMType)
	}
	for _, f := range l.Instance.Config.Firmware.Images {
		switch f.VMType {
		case "", limayaml.VZ:
			if f.Arch == *l.Instance.Config.Arch {
				return errors.New("`firmware.images` configuration is not supported for VZ driver")
			}
		}
	}
	if unknown := reflectutil.UnknownNonEmptyFields(l.Instance.Config, knownYamlProperties...); len(unknown) > 0 {
		logrus.Warnf("vmType %s: ignoring %+v", *l.Instance.Config.VMType, unknown)
	}

	if !limayaml.IsNativeArch(*l.Instance.Config.Arch) {
		return fmt.Errorf("unsupported arch: %q", *l.Instance.Config.Arch)
	}

	for k, v := range l.Instance.Config.CPUType {
		if v != "" {
			logrus.Warnf("vmType %s: ignoring cpuType[%q]: %q", *l.Instance.Config.VMType, k, v)
		}
	}

	for i, image := range l.Instance.Config.Images {
		if unknown := reflectutil.UnknownNonEmptyFields(image, "File", "Kernel", "Initrd"); len(unknown) > 0 {
			logrus.Warnf("vmType %s: ignoring images[%d]: %+v", *l.Instance.Config.VMType, i, unknown)
		}
	}

	for i, mount := range l.Instance.Config.Mounts {
		if unknown := reflectutil.UnknownNonEmptyFields(mount, "Location",
			"MountPoint",
			"Writable",
			"SSHFS",
			"NineP",
		); len(unknown) > 0 {
			logrus.Warnf("vmType %s: ignoring mounts[%d]: %+v", *l.Instance.Config.VMType, i, unknown)
		}
	}

	for i, network := range l.Instance.Config.Networks {
		if unknown := reflectutil.UnknownNonEmptyFields(network, "VZNAT",
			"Lima",
			"Socket",
			"MACAddress",
			"Metric",
			"Interface",
		); len(unknown) > 0 {
			logrus.Warnf("vmType %s: ignoring networks[%d]: %+v", *l.Instance.Config.VMType, i, unknown)
		}
	}

	switch audioDevice := *l.Instance.Config.Audio.Device; audioDevice {
	case "":
	case "vz", "default", "none":
	default:
		logrus.Warnf("field `audio.device` must be \"vz\", \"default\", or \"none\" for VZ driver, got %q", audioDevice)
	}

	switch videoDisplay := *l.Instance.Config.Video.Display; videoDisplay {
	case "vz", "default", "none":
	default:
		logrus.Warnf("field `video.display` must be \"vz\", \"default\", or \"none\" for VZ driver , got %q", videoDisplay)
	}
	return nil
}

func (l *LimaVzDriver) Initialize(_ context.Context) error {
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

func (l *LimaVzDriver) GuestAgentConn(_ context.Context) (net.Conn, error) {
	for _, socket := range l.machine.SocketDevices() {
		connect, err := socket.Connect(uint32(l.VSockPort))
		if err == nil && connect.SourcePort() != 0 {
			return connect, nil
		}
	}
	return nil, errors.New("unable to connect to guest agent via vsock port 2222")
}

func (l *LimaVzDriver) GetInfo() driver.Info {
	var info driver.Info
	if l.Instance != nil && l.Instance.Config != nil {
		info.CanRunGUI = l.canRunGUI()
	}

	info.DriverName = "vz"
	info.VsockPort = l.VSockPort
	info.VirtioPort = l.VirtioPort
	return info
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

func (l *LimaVzDriver) GetDisplayConnection(_ context.Context) (string, error) {
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
	return l.VSockPort == 0 && l.VirtioPort == ""
}
