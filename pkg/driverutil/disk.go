// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package driverutil

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/docker/go-units"
	"github.com/lima-vm/go-qcow2reader/image"
	"github.com/sirupsen/logrus"

	"github.com/lima-vm/lima/v2/pkg/downloader"
	"github.com/lima-vm/lima/v2/pkg/imgutil/proxyimgutil"
	"github.com/lima-vm/lima/v2/pkg/iso9660util"
	"github.com/lima-vm/lima/v2/pkg/limatype/filenames"
	"github.com/lima-vm/lima/v2/pkg/osutil"
)

// MigrateDiskLayout creates symlinks from the current filenames (disk, iso) to
// the legacy filenames (diffdisk, basedisk) used by older Lima versions.
// The original files are left in place so older Lima versions can still use them.
func MigrateDiskLayout(instDir string) error {
	diskPath := filepath.Join(instDir, filenames.Disk)
	if osutil.FileExists(diskPath) {
		return nil // already migrated or new instance
	}

	diffDiskPath := filepath.Join(instDir, filenames.DiffDiskLegacy)
	if osutil.FileExists(diffDiskPath) {
		logrus.Infof("Creating symlink %q -> %q", filenames.Disk, filenames.DiffDiskLegacy)
		if err := os.Symlink(filenames.DiffDiskLegacy, diskPath); err != nil {
			return fmt.Errorf("failed to symlink %q to %q: %w", filenames.Disk, filenames.DiffDiskLegacy, err)
		}
	}

	baseDiskPath := filepath.Join(instDir, filenames.BaseDiskLegacy)
	isoPath := filepath.Join(instDir, filenames.ISO)
	if osutil.FileExists(baseDiskPath) && !osutil.FileExists(isoPath) {
		isISO, err := iso9660util.IsISO9660(baseDiskPath)
		if err != nil {
			return err
		}
		if isISO {
			logrus.Infof("Creating symlink %q -> %q", filenames.ISO, filenames.BaseDiskLegacy)
			if err := os.Symlink(filenames.BaseDiskLegacy, isoPath); err != nil {
				return fmt.Errorf("failed to symlink %q to %q: %w", filenames.ISO, filenames.BaseDiskLegacy, err)
			}
		}
		// Non-ISO basedisk is a legacy qcow2 backing file; leave it for QEMU to resolve.
	}

	return nil
}

// EnsureDisk creates the VM disk from the downloaded image.
// For ISO images, it renames the image to "iso" and creates an empty disk.
// For non-ISO images, it converts the image to "disk" and removes the original.
func EnsureDisk(ctx context.Context, instDir, diskSize string, diskImageFormat image.Type) error {
	diskPath := filepath.Join(instDir, filenames.Disk)
	if _, err := os.Stat(diskPath); err == nil || !errors.Is(err, os.ErrNotExist) {
		return err
	}

	imagePath := filepath.Join(instDir, filenames.Image)
	isISO, err := iso9660util.IsISO9660(imagePath)
	if err != nil {
		return err
	}

	diskSizeInBytes, _ := units.RAMInBytes(diskSize)
	diskUtil := proxyimgutil.NewDiskUtil(ctx)

	if isISO {
		isoPath := filepath.Join(instDir, filenames.ISO)
		if err := os.Rename(imagePath, isoPath); err != nil {
			return err
		}
		f, err := os.Create(diskPath)
		if err != nil {
			_ = os.Rename(isoPath, imagePath)
			return err
		}
		if err := f.Close(); err != nil {
			os.Remove(diskPath)
			_ = os.Rename(isoPath, imagePath)
			return err
		}
		if err := diskUtil.Convert(ctx, diskImageFormat, diskPath, diskPath, &diskSizeInBytes, false); err != nil {
			os.Remove(diskPath)
			_ = os.Rename(isoPath, imagePath)
			return fmt.Errorf("failed to create disk %q: %w", diskPath, err)
		}
	} else {
		if downloader.IsRawImage(imagePath) {
			if err = os.Rename(imagePath, diskPath); err != nil {
				return err
			}
		} else {
			if err := diskUtil.Convert(ctx, diskImageFormat, imagePath, diskPath, &diskSizeInBytes, false); err != nil {
				return fmt.Errorf("failed to convert %q to %q: %w", imagePath, diskPath, err)
			}
			os.Remove(imagePath)
		}
	}
	return nil
}
