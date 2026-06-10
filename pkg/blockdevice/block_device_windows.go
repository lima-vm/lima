// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package blockdevice

import (
	"context"
	"errors"
	"io"
	"os"
)

// On Windows there is no fd-passing mechanism to hand a privately opened
// descriptor to the VM process, so Lima never opens host block devices
// itself. The backend (QEMU) opens the raw device path, e.g.
// \\.\PhysicalDriveN, directly with the privileges of the Lima process.

// Open is never called on Windows; the backend opens the device path itself.
func Open(_ context.Context, devicePath, _ string) (*os.File, error) {
	return nil, errors.New("lima does not open block devices itself on Windows; the backend opens " + devicePath + " directly")
}

// EnsureAccess is a no-op on Windows. Opening raw devices requires the Lima
// process to be sufficiently privileged (e.g. elevated); there is no sudo
// equivalent to pre-authorize here, so failures surface when the backend
// opens the device.
func EnsureAccess(_ context.Context, _ []string) error {
	return nil
}

// ServeSudoOpenBlockDevice exists so the hidden helper command is uniformly
// registered; the helper is never invoked on Windows because Open is never
// used here.
func ServeSudoOpenBlockDevice(_ io.Reader) error {
	return errors.New("the block-device helper is not used on Windows; the backend opens the device path directly")
}

// EnsureDeviceAccessible is never called on Windows; the backend opens the
// device path with the privileges of the Lima process itself.
func EnsureDeviceAccessible(_ context.Context, devicePath string) error {
	return errors.New("lima does not manage block device access on Windows; the backend opens " + devicePath + " directly")
}

// Sudoers has no equivalent on Windows, where privilege comes from running
// the Lima process elevated instead of from a sudoers rule.
func Sudoers(_ string) (string, error) {
	return "", errors.New("sudoers entries do not exist on Windows; run the Lima process elevated to open block devices")
}
