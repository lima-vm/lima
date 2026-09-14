// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package blockdevice

import (
	"errors"
	"fmt"
	"path"
	"regexp"
)

var macOSDiskDevicePathRE = regexp.MustCompile(`^/dev/r?disk\d+(s\d+)*$`)

// ValidateDiskDevicePath validates the host disk path grammar without accessing
// the device, so configuration validation can reject it before provisioning.
func ValidateDiskDevicePath(devicePath string) error {
	if devicePath == "" {
		return errors.New("devicePath must not be empty")
	}
	if !path.IsAbs(devicePath) {
		return fmt.Errorf("devicePath %q must be an absolute path", devicePath)
	}
	if path.Clean(devicePath) != devicePath {
		return fmt.Errorf("devicePath %q must be normalized", devicePath)
	}
	if !macOSDiskDevicePathRE.MatchString(devicePath) {
		return fmt.Errorf("devicePath %q must be a macOS disk device path like /dev/disk4 or /dev/rdisk4s1", devicePath)
	}
	return nil
}
