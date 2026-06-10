// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package blockdevice

import "fmt"

// qemuDriveArgs passes the raw device path, e.g. \\.\PhysicalDriveN, straight
// through for QEMU to open. There is no fd-passing mechanism in QEMU on
// Windows, so the Lima process must be allowed to open the device itself
// (e.g. by running elevated).
func qemuDriveArgs(devicePath string, _ int, driveID string) []string {
	return []string{
		"-drive",
		fmt.Sprintf("file=%s,format=raw,if=none,id=%s,file.driver=host_device", devicePath, driveID),
	}
}
