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

	"github.com/lima-vm/lima/v2/pkg/imgutil/proxyimgutil"
	"github.com/lima-vm/lima/v2/pkg/iso9660util"
	"github.com/lima-vm/lima/v2/pkg/limatype"
	"github.com/lima-vm/lima/v2/pkg/limatype/filenames"
)

// EnsureDiskRaw just converts baseDisk (can be qcow2) to diffDisk (raw), unless baseDisk is an ISO9660 image.
// Note that "diffDisk" is a misnomer, it is actually created as a full disk since Lima v2.1.
func EnsureDiskRaw(ctx context.Context, inst *limatype.Instance) error {
	diffDisk := filepath.Join(inst.Dir, filenames.DiffDisk)
	if _, err := os.Stat(diffDisk); err == nil || !errors.Is(err, os.ErrNotExist) {
		// disk is already ensured
		return err
	}

	diskUtil := proxyimgutil.NewDiskUtil(ctx)

	baseDisk := filepath.Join(inst.Dir, filenames.BaseDisk)

	diskSize, _ := units.RAMInBytes(*inst.Config.Disk)
	if diskSize == 0 {
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
	if err := diskUtil.ConvertToRaw(ctx, baseDisk, diffDisk, &diskSize, false); err != nil {
		return fmt.Errorf("failed to convert %q to a raw disk %q: %w", baseDisk, diffDisk, err)
	}
	if err := os.RemoveAll(baseDisk); err != nil {
		return err
	}
	return nil
}
