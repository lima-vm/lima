// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package proxyimgutil

import (
	"errors"
	"os"
	"os/exec"

	"github.com/lima-vm/lima/pkg/imgutil"
	"github.com/lima-vm/lima/pkg/imgutil/nativeimgutil"
	"github.com/lima-vm/lima/pkg/imgutil/qemuimgutil"
)

// ImageDiskManager is a proxy implementation of imgutil.ImageDiskManager that uses both QEMU and native image utilities.
type ImageDiskManager struct {
	qemu   imgutil.ImageDiskManager
	native imgutil.ImageDiskManager
}

// NewDiskUtil returns a new instance of ImageDiskManager that uses both QEMU and native image utilities.
func NewDiskUtil() imgutil.ImageDiskManager {
	return &ImageDiskManager{
		qemu:   &qemuimgutil.QemuImageUtil{DefaultFormat: qemuimgutil.QemuImgFormat},
		native: &nativeimgutil.NativeImageUtil{},
	}
}

// CreateDisk creates a new disk image with the specified size.
func (p *ImageDiskManager) CreateDisk(disk string, size int64) error {
	err := p.qemu.CreateDisk(disk, size)
	if err == nil {
		return nil
	}
	if errors.Is(err, exec.ErrNotFound) {
		return p.native.CreateDisk(disk, size)
	}
	return err
}

// ResizeDisk resizes an existing disk image to the specified size.
func (p *ImageDiskManager) ResizeDisk(disk string, size int64) error {
	err := p.qemu.ResizeDisk(disk, size)
	if err == nil {
		return nil
	}
	if errors.Is(err, exec.ErrNotFound) {
		return p.native.ResizeDisk(disk, size)
	}
	return err
}

// ConvertToRaw converts a disk image to raw format.
func (p *ImageDiskManager) ConvertToRaw(source, dest string, size *int64, allowSourceWithBackingFile bool) error {
	err := p.qemu.ConvertToRaw(source, dest, size, allowSourceWithBackingFile)
	if err == nil {
		return nil
	}
	if errors.Is(err, exec.ErrNotFound) {
		return p.native.ConvertToRaw(source, dest, size, allowSourceWithBackingFile)
	}
	return err
}

func (p *ImageDiskManager) MakeSparse(f *os.File, offset int64) error {
	err := p.qemu.MakeSparse(f, offset)
	if err == nil {
		return nil
	}
	if errors.Is(err, exec.ErrNotFound) {
		return p.native.MakeSparse(f, offset)
	}
	return err
}
