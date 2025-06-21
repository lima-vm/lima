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

type ProxyImageDiskManager struct {
	qemu   imgutil.ImageDiskManager
	native imgutil.ImageDiskManager
}

func NewProxyImageUtil() (imgutil.ImageDiskManager, *qemuimgutil.QemuInfoProvider) {
	return &ProxyImageDiskManager{
		qemu:   qemuimgutil.NewQemuImageUtil(),
		native: nativeimgutil.NewNativeImageUtil(),
	}, qemuimgutil.NewQemuInfoProvider()
}

func (p *ProxyImageDiskManager) CreateDisk(disk string, size int) error {
	if err := p.qemu.CreateDisk(disk, size); err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return p.native.CreateDisk(disk, size)
		}
		return err
	}
	return nil
}

func (p *ProxyImageDiskManager) ResizeDisk(disk string, size int) error {
	if err := p.qemu.ResizeDisk(disk, size); err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return p.native.ResizeDisk(disk, size)
		}
		return err
	}
	return nil
}

func (p *ProxyImageDiskManager) ConvertToRaw(source, dest string, size *int64, allowSourceWithBackingFile bool) error {
	if err := p.qemu.ConvertToRaw(source, dest, size, allowSourceWithBackingFile); err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return p.native.ConvertToRaw(source, dest, size, allowSourceWithBackingFile)
		}
		return err
	}
	return nil
}

func (p *ProxyImageDiskManager) MakeSparse(f *os.File, offset int64) error {
	if err := p.qemu.MakeSparse(f, offset); err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return p.native.MakeSparse(f, offset)
		}
		return err
	}
	return nil
}
