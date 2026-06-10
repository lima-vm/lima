//go:build unix && !darwin

// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package blockdevice

import "fmt"

// qemuDriveArgs passes the inherited descriptor through a QEMU fd set, the
// native fd-passing mechanism on Linux and the other Unix hosts. Descriptor
// passing is preferred over opening the path in QEMU because it works without
// QEMU having any permissions on the device node and without altering any
// host state. The explicit host_device driver is required because QEMU's
// plain file driver rejects non-regular files, and its filename-based driver
// probing cannot resolve /dev/fdset pseudo-paths. auto-read-only must be
// disabled: with it enabled QEMU looks for a read-only descriptor in the fd
// set first, but Lima only adds a read-write one.
func qemuDriveArgs(devicePath string, index int, driveID string) []string {
	fdSet := index + 1
	return []string{
		"-add-fd",
		fmt.Sprintf("fd={{ %s %q %d }},set=%d,opaque=%s", QEMUFDTemplateFunc, devicePath, index, fdSet, driveID),
		"-drive",
		fmt.Sprintf("file=/dev/fdset/%d,format=raw,if=none,id=%s,file.driver=host_device,file.auto-read-only=off", fdSet, driveID),
	}
}
