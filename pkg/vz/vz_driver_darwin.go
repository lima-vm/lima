//go:build darwin && !no_vz
// +build darwin,!no_vz

package vz

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"github.com/lima-vm/lima/pkg/reflectutil"

	"github.com/Code-Hex/vz/v3"

	"github.com/sirupsen/logrus"

	"github.com/lima-vm/lima/pkg/driver"
	"github.com/lima-vm/lima/pkg/limayaml"
)

const Enabled = true

type LimaVzDriver struct {
	*driver.BaseDriver

	machine *virtualMachineWrapper
}

func New(driver *driver.BaseDriver) *LimaVzDriver {
	return &LimaVzDriver{
		BaseDriver: driver,
	}
}

func (l *LimaVzDriver) Validate() error {
	// Calling NewEFIBootLoader to do required version check for latest APIs
	_, err := vz.NewEFIBootLoader()
	if errors.Is(err, vz.ErrUnsupportedOSVersion) {
		return fmt.Errorf("VZ driver requires macOS 13 or higher to run")
	}
	if *l.Yaml.MountType == limayaml.NINEP {
		return fmt.Errorf("field `mountType` must be %q or %q for VZ driver , got %q", limayaml.REVSSHFS, limayaml.VIRTIOFS, *l.Yaml.MountType)
	}
	if *l.Yaml.Firmware.LegacyBIOS {
		return fmt.Errorf("`firmware.legacyBIOS` configuration is not supported for VZ driver")
	}
	if unknown := reflectutil.UnknownNonEmptyFields(l.Yaml, "VMType",
		"Arch",
		"Images",
		"CPUs",
		"CPUType",
		"Memory",
		"Disk",
		"Mounts",
		"MountType",
		"SSH",
		"Firmware",
		"Provision",
		"Containerd",
		"GuestInstallPrefix",
		"Probes",
		"PortForwards",
		"Message",
		"Networks",
		"Env",
		"DNS",
		"HostResolver",
		"PropagateProxyEnv",
		"CACertificates",
		"Rosetta",
		"AdditionalDisks",
		"Audio",
		"Video",
		"OS",
		"Plain",
	); len(unknown) > 0 {
		logrus.Warnf("vmType %s: ignoring %+v", *l.Yaml.VMType, unknown)
	}

	if !limayaml.IsNativeArch(*l.Yaml.Arch) {
		return fmt.Errorf("unsupported arch: %q", *l.Yaml.Arch)
	}

	for k, v := range l.Yaml.CPUType {
		if v != "" {
			logrus.Warnf("vmType %s: ignoring cpuType[%q]: %q", *l.Yaml.VMType, k, v)
		}
	}

	for i, image := range l.Yaml.Images {
		if unknown := reflectutil.UnknownNonEmptyFields(image, "File"); len(unknown) > 0 {
			logrus.Warnf("vmType %s: ignoring images[%d]: %+v", *l.Yaml.VMType, i, unknown)
		}
	}

	for i, mount := range l.Yaml.Mounts {
		if unknown := reflectutil.UnknownNonEmptyFields(mount, "Location",
			"MountPoint",
			"Writable",
			"SSHFS",
			"NineP",
		); len(unknown) > 0 {
			logrus.Warnf("vmType %s: ignoring mounts[%d]: %+v", *l.Yaml.VMType, i, unknown)
		}
	}

	for i, network := range l.Yaml.Networks {
		if unknown := reflectutil.UnknownNonEmptyFields(network, "VZNAT",
			"Lima",
			"Socket",
			"MACAddress",
			"Interface",
		); len(unknown) > 0 {
			logrus.Warnf("vmType %s: ignoring networks[%d]: %+v", *l.Yaml.VMType, i, unknown)
		}
	}

	audioDevice := *l.Yaml.Audio.Device
	if audioDevice != "" && audioDevice != "vz" {
		logrus.Warnf("field `audio.device` must be %q for VZ driver , got %q", "vz", audioDevice)
	}

	switch videoDisplay := *l.Yaml.Video.Display; videoDisplay {
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

func (l *LimaVzDriver) CreateDisk() error {
	return EnsureDisk(l.BaseDriver)
}

func (l *LimaVzDriver) Start(ctx context.Context) (chan error, error) {
	logrus.Infof("Starting VZ (hint: to watch the boot progress, see %q)", filepath.Join(l.Instance.Dir, "serial*.log"))
	vm, errCh, err := startVM(ctx, l.BaseDriver)
	if err != nil {
		if errors.Is(err, vz.ErrUnsupportedOSVersion) {
			return nil, fmt.Errorf("vz driver requires macOS 13 or higher to run: %q", err)
		}
		return nil, err
	}
	l.machine = vm

	return errCh, nil
}

func (l *LimaVzDriver) CanRunGUI() bool {
	switch *l.Yaml.Video.Display {
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
	return fmt.Errorf("RunGUI is not support for the given driver '%s' and diplay '%s'", "vz", *l.Yaml.Video.Display)
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
