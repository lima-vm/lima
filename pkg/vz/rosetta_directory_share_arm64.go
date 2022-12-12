//go:build darwin && arm64 && !no_vz
// +build darwin,arm64,!no_vz

package vz

import (
	"fmt"

	"github.com/Code-Hex/vz/v3"
	"github.com/sirupsen/logrus"
)

func createRosettaDirectoryShareConfiguration() (*vz.VirtioFileSystemDeviceConfiguration, error) {
	config, err := vz.NewVirtioFileSystemDeviceConfiguration("vz-rosetta")
	if err != nil {
		return nil, fmt.Errorf("failed to create a new virtio file system configuration: %w", err)
	}
	availability := vz.LinuxRosettaDirectoryShareAvailability()
	switch availability {
	case vz.LinuxRosettaAvailabilityNotSupported:
		return nil, errRosettaUnsupported
	case vz.LinuxRosettaAvailabilityNotInstalled:
		logrus.Info("Installing rosetta...")
		if err := vz.LinuxRosettaDirectoryShareInstallRosetta(); err != nil {
			return nil, fmt.Errorf("failed to install rosetta: %w", err)
		}
		logrus.Info("Rosetta installation complete.")
	case vz.LinuxRosettaAvailabilityInstalled:
		// nothing to do
	}

	rosettaShare, err := vz.NewLinuxRosettaDirectoryShare()
	if err != nil {
		return nil, fmt.Errorf("failed to create a new rosetta directory share: %w", err)
	}
	config.SetDirectoryShare(rosettaShare)

	return config, nil
}
