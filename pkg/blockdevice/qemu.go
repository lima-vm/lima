// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package blockdevice

import "fmt"

// QEMUFDTemplateFunc is the name of the cmdline template function, applied by
// the QEMU driver at process start, that opens a host block device (through
// the privileged helper when direct access is denied) and exposes the
// retained descriptor to QEMU via ExtraFiles, evaluating to the QEMU-side fd
// number.
const QEMUFDTemplateFunc = "fd_blockdevice"

// QEMUPathTemplateFunc is the name of the cmdline template function, applied
// by the QEMU driver at process start, that makes a host block device node
// accessible to the user (through the privileged helper when needed) and
// evaluates to the device path for QEMU to open itself.
const QEMUPathTemplateFunc = "path_blockdevice"

// QEMUDriveArgs returns the QEMU arguments that attach the host block device
// as a virtio-blk drive. The drive arguments come from the per-OS
// qemuDriveArgs implementations; the serial gives the guest a stable
// /dev/disk/by-id/virtio-<id> path derived from the host device basename,
// matching the VZ driver behavior.
func QEMUDriveArgs(devicePath string, index int, virtioBlkDevice string) []string {
	driveID := fmt.Sprintf("blockdevice%d", index)
	args := qemuDriveArgs(devicePath, index, driveID)
	return append(args, "-device", fmt.Sprintf("%s,drive=%s,serial=%s", virtioBlkDevice, driveID, GuestDeviceIdentifier(devicePath)))
}
