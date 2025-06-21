// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package qemuimgutil

import (
	"fmt"
	"os"
)

// QemuImageUtil is the QEMU implementation of the proxyimgutil Interface.
type QemuImageUtil struct {
	// Default format to use when creating disks
	DefaultFormat string
}

// NewQemuImageUtil returns a new QemuImageUtil instance with "qcow2" as the default format.
func NewQemuImageUtil() *QemuImageUtil {
	return &QemuImageUtil{
		DefaultFormat: "qcow2",
	}
}

// CreateDisk creates a new disk image with the specified size.
func (q *QemuImageUtil) CreateDisk(disk string, size int) error {
	return createDisk(disk, q.DefaultFormat, size)
}

// ResizeDisk resizes an existing disk image to the specified size.
func (q *QemuImageUtil) ResizeDisk(disk string, size int) error {
	info, err := getInfo(disk)
	if err != nil {
		return fmt.Errorf("failed to get info for disk %q: %w", disk, err)
	}
	return resizeDisk(disk, info.Format, size)
}

// ConvertToRaw converts a disk image to raw format.
func (q *QemuImageUtil) ConvertToRaw(source, dest string, size *int64, allowSourceWithBackingFile bool) error {
	if !allowSourceWithBackingFile {
		info, err := getInfo(source)
		if err != nil {
			return fmt.Errorf("failed to get info for source disk %q: %w", source, err)
		}
		if info.BackingFilename != "" || info.FullBackingFilename != "" {
			return fmt.Errorf("qcow2 image %q has an unexpected backing file: %q", source, info.BackingFilename)
		}
	}

	if err := convertToRaw(source, dest); err != nil {
		return err
	}

	if size != nil {
		destInfo, err := getInfo(dest)
		if err != nil {
			return fmt.Errorf("failed to get info for converted disk %q: %w", dest, err)
		}

		if *size > destInfo.VSize {
			return resizeDisk(dest, "raw", int(*size))
		}
	}

	return nil
}

// MakeSparse is a stub implementation as the native package doesn't provide this functionality.
func (q *QemuImageUtil) MakeSparse(_ *os.File, _ int64) error {
	return nil
}

// QemuInfoProvider is the QEMU implementation of the proxyimgutil InfoProvider.
type QemuInfoProvider struct{}

// NewQemuInfoProvider returns a new QemuInfoProvider instance.
func NewQemuInfoProvider() *QemuInfoProvider {
	return &QemuInfoProvider{}
}

// GetInfo retrieves information about a disk image.
func (q *QemuInfoProvider) GetInfo(path string) (*Info, error) {
	qemuInfo, err := getInfo(path)
	if err != nil {
		return nil, err
	}

	return qemuInfo, nil
}

// AcceptableAsBasedisk checks if a disk image is acceptable as a base disk.
func (q *QemuInfoProvider) AcceptableAsBasedisk(info *Info) error {
	return acceptableAsBasedisk(info)
}
