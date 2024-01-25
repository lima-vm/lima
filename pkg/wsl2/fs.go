package wsl2

import (
	"context"
	"errors"
	"os"
	"path/filepath"

	"github.com/lima-vm/lima/pkg/driver"
	"github.com/lima-vm/lima/pkg/fileutils"
	"github.com/lima-vm/lima/pkg/store/filenames"
	"github.com/sirupsen/logrus"
)

// EnsureFs downloads the root fs.
func EnsureFs(ctx context.Context, driver *driver.BaseDriver) error {
	baseDisk := filepath.Join(driver.Instance.Dir, filenames.BaseDisk)
	if _, err := os.Stat(baseDisk); errors.Is(err, os.ErrNotExist) {
		var ensuredBaseDisk bool
		errs := make([]error, len(driver.Yaml.Images))
		for i, f := range driver.Yaml.Images {
			if _, err := fileutils.DownloadFile(ctx, baseDisk, f.File, true, "the image", *driver.Yaml.Arch); err != nil {
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
