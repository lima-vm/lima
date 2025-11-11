// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package driverutil

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/coreos/go-semver/semver"
	"github.com/docker/go-units"
	"github.com/sirupsen/logrus"

	"github.com/lima-vm/lima/v2/pkg/imgutil/proxyimgutil"
	"github.com/lima-vm/lima/v2/pkg/iso9660util"
	"github.com/lima-vm/lima/v2/pkg/limatype"
	"github.com/lima-vm/lima/v2/pkg/limatype/filenames"
	"github.com/lima-vm/lima/v2/pkg/osutil"
)

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

		err = diskUtil.MakeSparse(ctx, diffDiskF, 0)
		if err != nil {
			diffDiskF.Close()
			return fmt.Errorf("failed to create sparse diff disk %q: %w", diffDisk, err)
		}
		return diffDiskF.Close()
	}
	// Check whether to use ASIF format
	converter := diskUtil.ConvertToASIF
	if !determineUseASIF() {
		converter = diskUtil.ConvertToRaw
	}
	if err = converter(ctx, baseDisk, diffDisk, &diskSize, false); err != nil {
		return fmt.Errorf("failed to convert %q to a disk %q: %w", baseDisk, diffDisk, err)
	}
	return err
}

func determineUseASIF() bool {
	var useASIF bool
	if macOSProductVersion, err := osutil.ProductVersion(); err != nil {
		logrus.WithError(err).Warn("Failed to get macOS product version; using raw format instead of ASIF")
	} else if macOSProductVersion.LessThan(*semver.New("26.0.0")) {
		logrus.Infof("macOS version %q does not support ASIF format; using raw format instead", macOSProductVersion)
	} else {
		// TODO: change default to true,
		// if the conversion from ASIF to raw while preserving sparsity is implemented,
		// or if enough testing is done to confirm that interoperability issues won't happen with ASIF.
		useASIF = false
		// allow overriding via LIMA_VZ_ASIF environment variable
		if envVar := os.Getenv("LIMA_VZ_ASIF"); envVar != "" {
			if b, err := strconv.ParseBool(envVar); err != nil {
				logrus.WithError(err).Warnf("invalid LIMA_VZ_ASIF value %q", envVar)
			} else {
				useASIF = b
				uses := "ASIF"
				if !useASIF {
					uses = "raw"
				}
				logrus.Infof("LIMA_VZ_ASIF=%s; using %s format to diff disk", envVar, uses)
			}
		} else if useASIF {
			logrus.Info("using ASIF format for the disk image")
		} else {
			logrus.Info("using raw format for the disk image")
		}
	}
	return useASIF
}
