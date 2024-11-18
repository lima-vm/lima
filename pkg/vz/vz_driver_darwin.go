//go:build darwin && !no_vz

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
	"github.com/mitchellh/mapstructure"

	"github.com/sirupsen/logrus"

	"github.com/lima-vm/lima/pkg/driver"
	"github.com/lima-vm/lima/pkg/limayaml"
	"github.com/lima-vm/lima/pkg/osutil"
	"github.com/lima-vm/lima/pkg/reflectutil"
	"github.com/lima-vm/lima/pkg/store/filenames"
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
	"SaveOnStop",
	"SSH",
	"TimeZone",
	"UpgradePackages",
	"User",
	"Video",
	"VMType",
}

const Enabled = true

type LimaVzDriver struct {
	*driver.BaseDriver

	machine *virtualMachineWrapper

	// Runtime configuration
	config LimaVzDriverRuntimeConfig
}

type LimaVzDriverRuntimeConfig struct {
	// SaveOnStop is a flag to save the VM state on stop
	SaveOnStop bool `json:"saveOnStop"`
}

func New(driver *driver.BaseDriver) *LimaVzDriver {
	return &LimaVzDriver{
		BaseDriver: driver,
		config: LimaVzDriverRuntimeConfig{
			SaveOnStop: *driver.Instance.Config.SaveOnStop,
		},
	}
}

func (l *LimaVzDriver) Validate() error {
	// Calling NewEFIBootLoader to do required version check for latest APIs
	_, err := vz.NewEFIBootLoader()
	if errors.Is(err, vz.ErrUnsupportedOSVersion) {
		return fmt.Errorf("VZ driver requires macOS 13 or higher to run")
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
				return fmt.Errorf("`firmware.images` configuration is not supported for VZ driver")
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
		if unknown := reflectutil.UnknownNonEmptyFields(image, "File"); len(unknown) > 0 {
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
	_, err := getMachineIdentifier(l.BaseDriver)
	return err
}

func (l *LimaVzDriver) CreateDisk(ctx context.Context) error {
	return EnsureDisk(ctx, l.BaseDriver)
}

func (l *LimaVzDriver) Start(ctx context.Context) (chan error, error) {
	logrus.Infof("Starting VZ (hint: to watch the boot progress, see %q)", filepath.Join(l.Instance.Dir, "serial*.log"))
	vm, errCh, err := startVM(ctx, l.BaseDriver)
	if err != nil {
		if errors.Is(err, vz.ErrUnsupportedOSVersion) {
			return nil, fmt.Errorf("vz driver requires macOS 13 or higher to run: %w", err)
		}
		return nil, err
	}
	l.machine = vm

	return errCh, nil
}

func (l *LimaVzDriver) CanRunGUI() bool {
	switch *l.Instance.Config.Video.Display {
	case "vz", "default":
		return true
	default:
		return false
	}
}

func (l *LimaVzDriver) RunGUI() error {
	if l.CanRunGUI() {
		return l.machine.StartGraphicApplication(1920, 1200)
	}
	//nolint:revive // error-strings
	return fmt.Errorf("RunGUI is not supported for the given driver '%s' and display '%s'", "vz", *l.Instance.Config.Video.Display)
}

func (l *LimaVzDriver) RuntimeConfig(_ context.Context, config interface{}) (interface{}, error) {
	if config == nil {
		return l.config, nil
	}
	var newConfig LimaVzDriverRuntimeConfig
	err := mapstructure.Decode(config, &newConfig)
	if err != nil {
		return nil, err
	}
	if newConfig.SaveOnStop {
		if runtime.GOARCH != "arm64" {
			return nil, fmt.Errorf("saveOnStop is not supported on %s", runtime.GOARCH)
		} else if runtime.GOOS != "darwin" {
			return nil, fmt.Errorf("saveOnStop is not supported on %s", runtime.GOOS)
		} else if macOSProductVersion, err := osutil.ProductVersion(); err != nil {
			return nil, fmt.Errorf("failed to get macOS product version: %w", err)
		} else if macOSProductVersion.Major < 14 {
			return nil, fmt.Errorf("saveOnStop is not supported on macOS %d", macOSProductVersion.Major)
		}
		logrus.Info("VZ RuntimeConfiguration changed: SaveOnStop is enabled")
		l.config.SaveOnStop = true
	} else {
		logrus.Info("VZ RuntimeConfiguration changed: SaveOnStop is disabled")
		l.config.SaveOnStop = false
	}
	return l.config, nil
}

func (l *LimaVzDriver) Stop(_ context.Context) error {
	if l.config.SaveOnStop {
		machineStatePath := filepath.Join(l.Instance.Dir, filenames.VzMachineState)
		if err := saveVM(l.machine.VirtualMachine, machineStatePath); err != nil {
			logrus.WithError(err).Warn("Failed to save VZ. Falling back to shutdown")
		} else {
			return nil
		}
	}

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
	return nil, fmt.Errorf("unable to connect to guest agent via vsock port 2222")
}
