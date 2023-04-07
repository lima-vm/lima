package vz

import (
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

func EnsureDisk(driver *driver.BaseDriver) error {

	diffDisk := filepath.Join(driver.Instance.Dir, filenames.DiffDisk)
	if _, err := os.Stat(diffDisk); err == nil || !errors.Is(err, os.ErrNotExist) {
		// disk is already ensured
		return err
	}

	baseDisk := filepath.Join(driver.Instance.Dir, filenames.BaseDisk)
	if _, err := os.Stat(baseDisk); errors.Is(err, os.ErrNotExist) {
		var ensuredBaseDisk bool
		errs := make([]error, len(driver.Yaml.Images))
		for i, f := range driver.Yaml.Images {
			if _, err := fileutils.DownloadFile(baseDisk, f.File, true, "the image", *driver.Yaml.Arch); err != nil {
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
	diskSize, _ := units.RAMInBytes(*driver.Yaml.Disk)
	if diskSize == 0 {
		return nil
	}
	//TODO - Break qemu dependency
	isBaseDiskISO, err := iso9660util.IsISO9660(baseDisk)
	if err != nil {
		return err
	}
	args := []string{"create", "-f", "qcow2"}
	if !isBaseDiskISO {
		baseDiskFormat, err := imgutil.DetectFormat(baseDisk)
		if err != nil {
			return err
		}
		args = append(args, "-F", baseDiskFormat, "-b", baseDisk)
	}
	args = append(args, diffDisk, strconv.Itoa(int(diskSize)))
	cmd := exec.Command("qemu-img", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to run %v: %q: %w", cmd.Args, string(out), err)
	}
	if err = imgutil.ConvertToRaw(diffDisk, diffDisk); err != nil {
		return fmt.Errorf("cannot convert qcow2 to raw for vz driver")
	}
	return nil
}
