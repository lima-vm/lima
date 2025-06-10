package vbox

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"

	"github.com/docker/go-units"
	"github.com/lima-vm/lima/pkg/driver"
	"github.com/lima-vm/lima/pkg/fileutils"
	"github.com/lima-vm/lima/pkg/iso9660util"
	"github.com/lima-vm/lima/pkg/qemu/imgutil"
	"github.com/lima-vm/lima/pkg/store/filenames"
)

func EnsureDisk(ctx context.Context, driver *driver.BaseDriver) error {
	diffDisk := filepath.Join(driver.Instance.Dir, filenames.DiffDisk)
	if _, err := os.Stat(diffDisk); err == nil || !errors.Is(err, os.ErrNotExist) {
		// disk is already ensured
		return err
	}

	baseDisk := filepath.Join(driver.Instance.Dir, filenames.BaseDisk)
	if _, err := os.Stat(baseDisk); errors.Is(err, os.ErrNotExist) {
		var ensuredBaseDisk bool
		errs := make([]error, len(driver.Instance.Config.Images))
		for i, f := range driver.Instance.Config.Images {
			if _, err := fileutils.DownloadFile(ctx, baseDisk, f.File, true, "the image", *driver.Instance.Config.Arch); err != nil {
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
	diskSize, _ := units.RAMInBytes(*driver.Instance.Config.Disk)
	if diskSize == 0 {
		return nil
	}
	// TODO - Break qemu dependency
	isBaseDiskISO, err := iso9660util.IsISO9660(baseDisk)
	if err != nil {
		return err
	}
	baseDiskInfo, err := imgutil.GetInfo(baseDisk)
	if err != nil {
		return fmt.Errorf("failed to get the information of base disk %q: %w", baseDisk, err)
	}
	if err = imgutil.AcceptableAsBasedisk(baseDiskInfo); err != nil {
		return fmt.Errorf("file %q is not acceptable as the base disk: %w", baseDisk, err)
	}
	if baseDiskInfo.Format == "" {
		return fmt.Errorf("failed to inspect the format of %q", baseDisk)
	}
	args := []string{"create", "-f", "qcow2"}
	if !isBaseDiskISO {
		args = append(args, "-F", baseDiskInfo.Format, "-b", baseDisk)
	}
	args = append(args, diffDisk, strconv.Itoa(int(diskSize)))
	cmd := exec.Command("qemu-img", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to run %v: %q: %w", cmd.Args, string(out), err)
	}
	if isBaseDiskISO {
		if err = os.Rename(baseDisk, baseDisk+".iso"); err != nil {
			return err
		}
		if err = os.Symlink(filenames.BaseDisk+".iso", baseDisk); err != nil {
			return err
		}
	} else {
		if err = os.Rename(baseDisk, baseDisk+".img"); err != nil {
			return err
		}
		if err = os.Symlink(filenames.BaseDisk+".img", baseDisk); err != nil {
			return err
		}
	}
	if err = imgutil.ConvertToVDI(diffDisk, diffDisk+".vdi"); err != nil {
		return errors.New("cannot convert qcow2 to vdi for vbox driver")
	}
	if err = os.Remove(diffDisk); err != nil {
		return err
	}
	return os.Symlink(filenames.DiffDisk+".vdi", diffDisk)
}
