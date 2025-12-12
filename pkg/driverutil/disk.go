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

	"github.com/lima-vm/lima/v2/pkg/imgutil/proxyimgutil"
	"github.com/lima-vm/lima/v2/pkg/iso9660util"
	"github.com/lima-vm/lima/v2/pkg/limatype/filenames"
)

// EnsureDisk ensures that the diff disk exists with the specified size and format.
// EnsureDisk usually just converts baseDisk (can be qcow2) to diffDisk (raw), unless baseDisk is an ISO9660 image.
// Note that "diffDisk" is a misnomer, it is actually created as a full disk since Lima v2.1.
func EnsureDisk(ctx context.Context, instDir, diskSize string, diskImageFormat image.Type) error {
	diffDisk := filepath.Join(instDir, filenames.DiffDisk)
	if _, err := os.Stat(diffDisk); err == nil || !errors.Is(err, os.ErrNotExist) {
		// disk is already ensured
		return err
	}

	diskUtil := proxyimgutil.NewDiskUtil(ctx)

	baseDisk := filepath.Join(instDir, filenames.BaseDisk)

	diskSizeInBytes, _ := units.RAMInBytes(diskSize)
	if diskSizeInBytes == 0 {
		return nil
	}
	var isBaseDiskISO bool
	if _, err := os.Stat(baseDisk); !errors.Is(err, os.ErrNotExist) {
		isBaseDiskISO, err = iso9660util.IsISO9660(baseDisk)
		if err != nil {
			return err
		}
		if isBaseDiskISO {
			// Create an empty data volume (sparse)
			diffDiskF, err := os.Create(diffDisk)
			if err != nil {
				return err
			}

			err = diskUtil.MakeSparse(ctx, diffDiskF, 0)
			if err != nil {
				diffDiskF.Close()
				return fmt.Errorf("failed to create sparse diff disk %q: %w", diffDisk, err)
			}
			return diffDiskF.Close()
		}
	}
	// "diffdisk" is a misnomer, it is actually created as a full disk since Lima v2.1.
	// Check whether to use ASIF format

	if err := diskUtil.Convert(ctx, diskImageFormat, baseDisk, diffDisk, &diskSizeInBytes, false); err != nil {
		return fmt.Errorf("failed to convert %q to a disk %q: %w", baseDisk, diffDisk, err)
	}
	if err := os.RemoveAll(baseDisk); err != nil {
		return err
	}
	return nil
}
