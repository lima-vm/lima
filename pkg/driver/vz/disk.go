// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package vz

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/docker/go-units"
	"github.com/lima-vm/lima/pkg/imgutil/proxyimgutil"
	"github.com/lima-vm/lima/pkg/iso9660util"
	"github.com/lima-vm/lima/pkg/store"
	"github.com/lima-vm/lima/pkg/store/filenames"
)

func EnsureDisk(ctx context.Context, inst *store.Instance) error {
	diffDisk := filepath.Join(inst.Dir, filenames.DiffDisk)
	if _, err := os.Stat(diffDisk); err == nil || !errors.Is(err, os.ErrNotExist) {
		// disk is already ensured
		return err
	}

	diskUtil := proxyimgutil.NewDiskUtil()

	baseDisk := filepath.Join(inst.Dir, filenames.BaseDisk)

	diskSize, _ := units.RAMInBytes(*inst.Config.Disk)
	if diskSize == 0 {
		return nil
	}
	isBaseDiskISO, err := iso9660util.IsISO9660(baseDisk)
	if err != nil {
		return err
	}
	if isBaseDiskISO {
		// Create an empty data volume (sparse)
		diffDiskF, err := os.Create(diffDisk)
		if err != nil {
			return err
		}

		err = diskUtil.MakeSparse(diffDiskF, 0)
		if err != nil {
			diffDiskF.Close()
			return fmt.Errorf("failed to create sparse diff disk %q: %w", diffDisk, err)
		}
		return diffDiskF.Close()
	}
	if err = diskUtil.ConvertToRaw(baseDisk, diffDisk, &diskSize, false); err != nil {
		return fmt.Errorf("failed to convert %q to a raw disk %q: %w", baseDisk, diffDisk, err)
	}
	return err
}
