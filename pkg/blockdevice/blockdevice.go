// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

// Package blockdevice attaches host block devices to Lima VMs. On Unix hosts
// Lima opens the device itself, directly when the user has access or through
// a privileged sudo helper otherwise, and hands the descriptor to the
// backend. On Windows the backend opens the raw device path directly.
package blockdevice

import (
	"path/filepath"
	"strings"
)

// SudoOpenBlockDeviceCommand is the hidden limactl helper command that opens
// a host block device as root and passes the descriptor back to the
// unprivileged process.
const SudoOpenBlockDeviceCommand = "sudo-open-block-device"

// GuestDeviceIdentifier derives a guest block device identifier from the host
// device basename. This does not force the guest kernel to use a specific
// /dev node like "/dev/vdb", but it does give the guest a stable identifier
// value that Linux exposes in predictable places such as /dev/disk/by-id and
// lsblk SERIAL output. The identifier is truncated to 20 characters, the
// limit shared by the VZ block device identifier and the virtio-blk serial.
func GuestDeviceIdentifier(devicePath string) string {
	// Windows DOS device paths (\\.\PhysicalDrive2, or the equivalent
	// //./PhysicalDrive2 form that survives MSYS2 shells) are treated by
	// filepath as volume names rather than directories, so the device name
	// must be extracted before taking the basename.
	for _, prefix := range []string{`\\.\`, `//./`} {
		if rest, ok := strings.CutPrefix(devicePath, prefix); ok {
			devicePath = rest
			break
		}
	}
	base := filepath.Base(devicePath)
	var b strings.Builder
	b.Grow(len(base))
	for _, r := range base {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-', r == '_', r == '.':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}
	id := b.String()
	if id == "" {
		id = "block-device"
	}
	if len(id) > 20 {
		id = id[:20]
	}
	return id
}
