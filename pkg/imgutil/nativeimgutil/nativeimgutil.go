// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

// Package nativeimgutil provides image utilities that do not depend on `qemu-img` binary.
package nativeimgutil

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	containerdfs "github.com/containerd/continuity/fs"
	"github.com/docker/go-units"
	"github.com/lima-vm/go-qcow2reader"
	"github.com/lima-vm/go-qcow2reader/convert"
	"github.com/lima-vm/go-qcow2reader/image/qcow2"
	"github.com/lima-vm/go-qcow2reader/image/raw"
	"github.com/sirupsen/logrus"

	"github.com/lima-vm/lima/pkg/progressbar"
)

// Disk image size must be aligned to sector size. Qemu block layer is rounding
// up the size to 512 bytes. Apple virtualization framework reject disks not
// aligned to 512 bytes.
const sectorSize = 512

// NativeImageUtil is the native implementation of the imgutil.ImageDiskManager.
type NativeImageUtil struct{}

// roundUp rounds size up to sectorSize.
func roundUp(size int64) int64 {
	sectors := (size + sectorSize - 1) / sectorSize
	return sectors * sectorSize
}

// convertToRaw converts a source disk into a raw disk.
// source and dest may be same.
// convertToRaw is a NOP if source == dest, and no resizing is needed.
func convertToRaw(source, dest string, size *int64, allowSourceWithBackingFile bool) error {
	srcF, err := os.Open(source)
	if err != nil {
		return err
	}
	defer srcF.Close()
	srcImg, err := qcow2reader.Open(srcF)
	if err != nil {
		return fmt.Errorf("failed to detect the format of %q: %w", source, err)
	}
	if size != nil && *size < srcImg.Size() {
		return fmt.Errorf("specified size %d is smaller than the original image size (%d) of %q", *size, srcImg.Size(), source)
	}
	logrus.Infof("Converting %q (%s) to a raw disk %q", source, srcImg.Type(), dest)
	switch t := srcImg.Type(); t {
	case raw.Type:
		if err = srcF.Close(); err != nil {
			return err
		}
		return convertRawToRaw(source, dest, size)
	case qcow2.Type:
		if !allowSourceWithBackingFile {
			q, ok := srcImg.(*qcow2.Qcow2)
			if !ok {
				return fmt.Errorf("unexpected qcow2 image %T", srcImg)
			}
			if q.BackingFile != "" {
				return fmt.Errorf("qcow2 image %q has an unexpected backing file: %q", source, q.BackingFile)
			}
		}
	default:
		logrus.Warnf("image %q has an unexpected format: %q", source, t)
	}
	if err = srcImg.Readable(); err != nil {
		return fmt.Errorf("image %q is not readable: %w", source, err)
	}

	// Create a tmp file because source and dest can be same.
	destTmpF, err := os.CreateTemp(filepath.Dir(dest), filepath.Base(dest)+".lima-*.tmp")
	if err != nil {
		return err
	}
	destTmp := destTmpF.Name()
	defer os.RemoveAll(destTmp)
	defer destTmpF.Close()

	// Truncating before copy eliminates the seeks during copy and provide a
	// hint to the file system that may minimize allocations and fragmentation
	// of the file.
	if err := makeSparse(destTmpF, srcImg.Size()); err != nil {
		return err
	}

	// Copy
	bar, err := progressbar.New(srcImg.Size())
	if err != nil {
		return err
	}
	bar.Start()
	err = convert.Convert(destTmpF, srcImg, convert.Options{Progress: bar})
	bar.Finish()
	if err != nil {
		return fmt.Errorf("failed to convert image: %w", err)
	}

	// Resize
	if size != nil {
		logrus.Infof("Expanding to %s", units.BytesSize(float64(*size)))
		if err = makeSparse(destTmpF, *size); err != nil {
			return err
		}
	}
	if err = destTmpF.Close(); err != nil {
		return err
	}

	// Rename destTmp into dest
	if err = os.RemoveAll(dest); err != nil {
		return err
	}
	return os.Rename(destTmp, dest)
}

func convertRawToRaw(source, dest string, size *int64) error {
	if source != dest {
		// continuity attempts clonefile
		if err := containerdfs.CopyFile(dest, source); err != nil {
			return fmt.Errorf("failed to copy %q into %q: %w", source, dest, err)
		}
	}
	if size != nil {
		logrus.Infof("Expanding to %s", units.BytesSize(float64(*size)))
		destF, err := os.OpenFile(dest, os.O_RDWR, 0o644)
		if err != nil {
			return err
		}
		if err = makeSparse(destF, *size); err != nil {
			_ = destF.Close()
			return err
		}
		return destF.Close()
	}
	return nil
}

func makeSparse(f *os.File, offset int64) error {
	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		return err
	}
	return f.Truncate(offset)
}

// CreateDisk creates a new disk image with the specified size.
func (n *NativeImageUtil) CreateDisk(disk string, size int64) error {
	if _, err := os.Stat(disk); err == nil || !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	f, err := os.Create(disk)
	if err != nil {
		return err
	}
	defer f.Close()
	roundedSize := roundUp(size)
	return f.Truncate(int64(roundedSize))
}

// ConvertToRaw converts a disk image to raw format.
func (n *NativeImageUtil) ConvertToRaw(source, dest string, size *int64, allowSourceWithBackingFile bool) error {
	return convertToRaw(source, dest, size, allowSourceWithBackingFile)
}

// ResizeDisk resizes an existing disk image to the specified size.
func (n *NativeImageUtil) ResizeDisk(disk string, size int64) error {
	roundedSize := roundUp(size)
	return os.Truncate(disk, roundedSize)
}

// MakeSparse makes a file sparse, starting from the specified offset.
func (n *NativeImageUtil) MakeSparse(f *os.File, offset int64) error {
	return makeSparse(f, offset)
}
