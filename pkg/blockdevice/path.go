// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package blockdevice

import (
	"errors"
	"fmt"
	"path"
	"regexp"
	"strings"
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

// DevicesOverlap compares validated macOS disk paths. Two writable attachments
// to the same storage can corrupt it even within one VM; disjoint partitions
// may share a whole-disk lock, but raw aliases and parent/child nodes may not.
func DevicesOverlap(a, b string) bool {
	a, b = strings.TrimPrefix(path.Base(a), "r"), strings.TrimPrefix(path.Base(b), "r")
	// Include the partition separator so disk4 does not match disk40, nor s1 s10.
	return a == b || strings.HasPrefix(a, b+"s") || strings.HasPrefix(b, a+"s")
}
