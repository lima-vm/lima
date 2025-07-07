// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package wsl2

import (
	"context"
	"errors"
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"

	"github.com/lima-vm/lima/pkg/fileutils"
	"github.com/lima-vm/lima/pkg/store"
	"github.com/lima-vm/lima/pkg/store/filenames"
)

// EnsureFs downloads the root fs.
func EnsureFs(ctx context.Context, inst *store.Instance) error {
	baseDisk := filepath.Join(inst.Dir, filenames.BaseDisk)
	if _, err := os.Stat(baseDisk); errors.Is(err, os.ErrNotExist) {
		var ensuredBaseDisk bool
		errs := make([]error, len(inst.Config.Images))
		for i, f := range inst.Config.Images {
			if _, err := fileutils.DownloadFile(ctx, baseDisk, f.File, true, "the image", *inst.Config.Arch); err != nil {
				errs[i] = err
				continue
			}
			ensuredBaseDisk = true
			break
		}
		if !ensuredBaseDisk {
			return fileutils.Errors(errs)
		}
	}
	logrus.Info("Download succeeded")

	return nil
}
