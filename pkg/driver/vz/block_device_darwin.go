//go:build darwin && !no_vz

// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package vz

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	vzapi "github.com/Code-Hex/vz/v3"

	"github.com/lima-vm/lima/v2/pkg/blockdevice"
	"github.com/lima-vm/lima/v2/pkg/limatype"
)

// attachHostBlockDevices runs while Lima is building the VZ storage device
// configuration before the VM boots. For each configured host block device, it
// obtains a retained descriptor from the privileged helper path, wraps that
// descriptor in a VZ block-device attachment, and appends it to the VM's
// storage device list alongside the normal disk image attachments.
func attachHostBlockDevices(ctx context.Context, inst *limatype.Instance, configurations []vzapi.StorageDeviceConfiguration) ([]vzapi.StorageDeviceConfiguration, error) {
	if len(inst.Config.BlockDevices) == 0 {
		return configurations, nil
	}

	for i, devicePath := range inst.Config.BlockDevices {
		deviceFile, err := openHostBlockDevice(ctx, inst, devicePath, i)
		if err != nil {
			return nil, err
		}
		attachment, err := vzapi.NewDiskBlockDeviceStorageDeviceAttachment(deviceFile, false, vzapi.DiskSynchronizationModeFull)
		if err != nil {
			return nil, fmt.Errorf("failed to create block device attachment for %q: %w", devicePath, err)
		}
		device, err := vzapi.NewVirtioBlockDeviceConfiguration(attachment)
		if err != nil {
			return nil, fmt.Errorf("failed to create virtio block device for %q: %w", devicePath, err)
		}
		if err := device.SetBlockDeviceIdentifier(guestBlockDeviceIdentifier(devicePath)); err != nil {
			return nil, fmt.Errorf("failed to set block device identifier for %q: %w", devicePath, err)
		}
		configurations = append(configurations, device)
	}
	return configurations, nil
}

// openHostBlockDevice bridges the generic host helper into the VZ lifecycle by
// choosing a VZ-local socket path and retaining the returned descriptor until
// the VM stops.
func openHostBlockDevice(ctx context.Context, inst *limatype.Instance, devicePath string, index int) (*os.File, error) {
	socketPath := filepath.Join(inst.Dir, fmt.Sprintf("vz-block-device.%d.sock", index))
	deviceFile, err := blockdevice.Open(ctx, devicePath, socketPath)
	if err != nil {
		return nil, err
	}
	vmRetainedFileDescriptors = append(vmRetainedFileDescriptors, deviceFile)
	return deviceFile, nil
}

// guestBlockDeviceIdentifier derives the VZ block device identifier from the
// host device basename. This does not force the guest kernel to use a specific
// /dev node like "/dev/vdb", but it does give the guest a stable identifier
// value that Linux exposes in predictable places such as /dev/disk/by-id and
// lsblk SERIAL output.
func guestBlockDeviceIdentifier(devicePath string) string {
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
