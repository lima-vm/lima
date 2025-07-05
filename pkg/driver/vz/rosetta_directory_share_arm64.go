//go:build darwin && arm64 && !no_vz

// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package vz

import (
	"fmt"

	"github.com/Code-Hex/vz/v3"
	"github.com/coreos/go-semver/semver"
	"github.com/sirupsen/logrus"

	"github.com/lima-vm/lima/pkg/osutil"
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
		logrus.Info("Hint: try `softwareupdate --install-rosetta` if Lima gets stuck here")
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
	macOSProductVersion, err := osutil.ProductVersion()
	if err != nil {
		return nil, fmt.Errorf("failed to get macOS product version: %w", err)
	}
	if !macOSProductVersion.LessThan(*semver.New("14.0.0")) {
		cachingOption, err := vz.NewLinuxRosettaAbstractSocketCachingOptions("rosetta")
		if err != nil {
			return nil, fmt.Errorf("failed to create a new rosetta directory share caching option: %w", err)
		}
		rosettaShare.SetOptions(cachingOption)
	}
	config.SetDirectoryShare(rosettaShare)

	return config, nil
}
