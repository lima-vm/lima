// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package proxyimgutil

import (
	"context"
	"errors"
	"os"
	"os/exec"

	"github.com/lima-vm/go-qcow2reader/image"
	"github.com/lima-vm/go-qcow2reader/image/raw"

	"github.com/lima-vm/lima/v2/pkg/imgutil"
	"github.com/lima-vm/lima/v2/pkg/imgutil/nativeimgutil"
	"github.com/lima-vm/lima/v2/pkg/qemuimgutil"
)

// ImageDiskManager is a proxy implementation of imgutil.ImageDiskManager that uses both QEMU and native image utilities.
type ImageDiskManager struct {
	qemu   imgutil.ImageDiskManager
	native imgutil.ImageDiskManager
}

// NewDiskUtil returns a new instance of ImageDiskManager that uses both QEMU and native image utilities.
func NewDiskUtil(_ context.Context) imgutil.ImageDiskManager {
	return &ImageDiskManager{
		qemu:   &qemuimgutil.QemuImageUtil{DefaultFormat: qemuimgutil.QemuImgFormat},
		native: &nativeimgutil.NativeImageUtil{},
	}
}

// CreateDisk creates a new disk image with the specified size.
func (p *ImageDiskManager) CreateDisk(ctx context.Context, disk string, size int64) error {
	err := p.qemu.CreateDisk(ctx, disk, size)
	if err == nil {
		return nil
	}
	if errors.Is(err, exec.ErrNotFound) {
		return p.native.CreateDisk(ctx, disk, size)
	}
	return err
}

// ResizeDisk resizes an existing disk image to the specified size.
func (p *ImageDiskManager) ResizeDisk(ctx context.Context, disk string, size int64) error {
	err := p.qemu.ResizeDisk(ctx, disk, size)
	if err == nil {
		return nil
	}
	if errors.Is(err, exec.ErrNotFound) {
		return p.native.ResizeDisk(ctx, disk, size)
	}
	return err
}

// Convert converts a disk image to the specified format.
// Currently supported formats are raw.Type and asif.Type.
func (p *ImageDiskManager) Convert(ctx context.Context, imageType image.Type, source, dest string, size *int64, allowSourceWithBackingFile bool) error {
	if imageType == raw.Type {
		err := p.qemu.Convert(ctx, imageType, source, dest, size, allowSourceWithBackingFile)
		if err == nil {
			return nil
		}
		if errors.Is(err, exec.ErrNotFound) {
			return p.native.Convert(ctx, imageType, source, dest, size, allowSourceWithBackingFile)
		}
		return err
	}
	return p.native.Convert(ctx, imageType, source, dest, size, allowSourceWithBackingFile)
}

func (p *ImageDiskManager) MakeSparse(ctx context.Context, f *os.File, offset int64) error {
	err := p.qemu.MakeSparse(ctx, f, offset)
	if err == nil {
		return nil
	}
	if errors.Is(err, exec.ErrNotFound) {
		return p.native.MakeSparse(ctx, f, offset)
	}
	return err
}
