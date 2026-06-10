// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package blockdevice

import "fmt"

// qemuDriveArgs lets QEMU open the device path itself, after the
// QEMUPathTemplateFunc template made the node accessible to the user. macOS
// offers no way to hand a privately opened descriptor to QEMU: opening
// /dev/fd/N re-checks the device node permissions, and fcntl F_SETFL fails
// with ENOTTY on disk descriptors, which breaks QEMU's /dev/fdset
// duplication.
func qemuDriveArgs(devicePath string, index int, driveID string) []string {
	return []string{
		"-drive",
		fmt.Sprintf("file={{ %s %q %d }},format=raw,if=none,id=%s,file.driver=host_device", QEMUPathTemplateFunc, devicePath, index, driveID),
	}
}
