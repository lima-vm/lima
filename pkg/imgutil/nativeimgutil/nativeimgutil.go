// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

// Package nativeimgutil provides image utilities that do not depend on `qemu-img` binary.
package nativeimgutil

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"math"
	"math/rand/v2"
	"os"
	"path/filepath"

	containerdfs "github.com/containerd/continuity/fs"
	"github.com/docker/go-units"
	"github.com/lima-vm/go-qcow2reader"
	"github.com/lima-vm/go-qcow2reader/convert"
	"github.com/lima-vm/go-qcow2reader/image/asif"
	"github.com/lima-vm/go-qcow2reader/image/qcow2"
	"github.com/lima-vm/go-qcow2reader/image/raw"
	"github.com/sirupsen/logrus"

	"github.com/lima-vm/lima/v2/pkg/imgutil/nativeimgutil/asifutil"
	"github.com/lima-vm/lima/v2/pkg/progressbar"
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

type targetImageType string

const (
	imageRaw  targetImageType = "raw"
	imageASIF targetImageType = "ASIF"
)

// convertTo converts a source disk into a raw or ASIF disk.
// source and dest may be same.
// convertTo is a NOP if source == dest, and no resizing is needed.
func convertTo(destType targetImageType, source, dest string, size *int64, allowSourceWithBackingFile bool) error {
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
	logrus.Infof("Converting %q (%s) to a %s disk %q", source, srcImg.Type(), destType, dest)
	switch t := srcImg.Type(); t {
	case raw.Type:
		if err = srcF.Close(); err != nil {
			return err
		}
		if destType == imageRaw {
			return convertRawToRaw(source, dest, size)
		}
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
	case asif.Type:
		if destType == imageASIF {
			return convertASIFToASIF(source, dest, size)
		}
		return fmt.Errorf("conversion from ASIF to %q is not supported", destType)
	default:
		logrus.Warnf("image %q has an unexpected format: %q", source, t)
	}
	if err = srcImg.Readable(); err != nil {
		return fmt.Errorf("image %q is not readable: %w", source, err)
	}

	// Create a tmp file because source and dest can be same.
	var (
		destTmpF       *os.File
		destTmp        string
		attachedDevice string
	)
	switch destType {
	case imageRaw:
		destTmpF, err = os.CreateTemp(filepath.Dir(dest), filepath.Base(dest)+".lima-*.tmp")
		destTmp = destTmpF.Name()
	case imageASIF:
		// destTmp != destTmpF.Name() because destTmpF is mounted ASIF device file.
		randomBase := fmt.Sprintf("%s.lima-%d.tmp.asif", filepath.Base(dest), rand.UintN(math.MaxUint))
		destTmp = filepath.Join(filepath.Dir(dest), randomBase)
		attachedDevice, destTmpF, err = asifutil.NewAttachedASIF(destTmp, srcImg.Size())
	default:
		return fmt.Errorf("unsupported target image type: %q", destType)
	}
	if err != nil {
		return err
	}
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
	// Detach ASIF device
	if destType == imageASIF {
		err := asifutil.DetachASIF(attachedDevice)
		if err != nil {
			return fmt.Errorf("failed to detach ASIF image %q: %w", attachedDevice, err)
		}
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
		if err := os.Chmod(dest, 0o644); err != nil {
			return fmt.Errorf("failed to set permissions on %q: %w", dest, err)
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

func convertASIFToASIF(source, dest string, size *int64) error {
	if source != dest {
		if err := containerdfs.CopyFile(dest, source); err != nil {
			return fmt.Errorf("failed to copy %q into %q: %w", source, dest, err)
		}
		if err := os.Chmod(dest, 0o644); err != nil {
			return fmt.Errorf("failed to set permissions on %q: %w", dest, err)
		}
	}
	if size != nil {
		logrus.Infof("Resizing to %s", units.BytesSize(float64(*size)))
		if err := asifutil.ResizeASIF(dest, *size); err != nil {
			return fmt.Errorf("failed to resize ASIF image %q: %w", dest, err)
		}
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
func (n *NativeImageUtil) CreateDisk(_ context.Context, disk string, size int64) error {
	if _, err := os.Stat(disk); err == nil || !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	f, err := os.Create(disk)
	if err != nil {
		return err
	}
	defer f.Close()
	roundedSize := roundUp(size)
	return f.Truncate(roundedSize)
}

// ConvertToRaw converts a disk image to raw format.
func (n *NativeImageUtil) ConvertToRaw(_ context.Context, source, dest string, size *int64, allowSourceWithBackingFile bool) error {
	return convertTo(imageRaw, source, dest, size, allowSourceWithBackingFile)
}

// ResizeDisk resizes an existing disk image to the specified size.
func (n *NativeImageUtil) ResizeDisk(_ context.Context, disk string, size int64) error {
	roundedSize := roundUp(size)
	return os.Truncate(disk, roundedSize)
}

// MakeSparse makes a file sparse, starting from the specified offset.
func (n *NativeImageUtil) MakeSparse(_ context.Context, f *os.File, offset int64) error {
	return makeSparse(f, offset)
}

// ConvertToASIF converts a disk image to ASIF format.
func (n *NativeImageUtil) ConvertToASIF(_ context.Context, source, dest string, size *int64, allowSourceWithBackingFile bool) error {
	return convertTo(imageASIF, source, dest, size, allowSourceWithBackingFile)
}
