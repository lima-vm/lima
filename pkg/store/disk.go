// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package store

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/lima-vm/go-qcow2reader"
	"github.com/lima-vm/lima/pkg/qemu/imgutil"
	"github.com/lima-vm/lima/pkg/store/filenames"
)

type Disk struct {
	Name        string `json:"name"`
	Size        int64  `json:"size"`
	Format      string `json:"format"`
	Dir         string `json:"dir"`
	Instance    string `json:"instance"`
	InstanceDir string `json:"instanceDir"`
	MountPoint  string `json:"mountPoint"`
}

func InspectDisk(diskName string) (*Disk, error) {
	disk := &Disk{
		Name: diskName,
	}

	diskDir, err := DiskDir(diskName)
	if err != nil {
		return nil, err
	}

	disk.Dir = diskDir
	dataDisk := filepath.Join(diskDir, filenames.DataDisk)
	if _, err := os.Stat(dataDisk); err != nil {
		return nil, err
	}

	disk.Size, disk.Format, err = inspectDisk(dataDisk)
	if err != nil {
		return nil, err
	}

	instDir, err := os.Readlink(filepath.Join(diskDir, filenames.InUseBy))
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			return nil, err
		}
	} else {
		disk.Instance = filepath.Base(instDir)
		disk.InstanceDir = instDir
	}

	disk.MountPoint = fmt.Sprintf("/mnt/lima-%s", diskName)

	return disk, nil
}

// inspectDisk attempts to inspect the disk size and format by itself,
// and falls back to inspectDiskWithQemuImg on an error.
func inspectDisk(fName string) (size int64, format string, _ error) {
	f, err := os.Open(fName)
	if err != nil {
		return inspectDiskWithQemuImg(fName)
	}
	defer f.Close()
	img, err := qcow2reader.Open(f)
	if err != nil {
		return inspectDiskWithQemuImg(fName)
	}
	sz := img.Size()
	if sz < 0 {
		return inspectDiskWithQemuImg(fName)
	}

	return sz, string(img.Type()), nil
}

// inspectDiskSizeWithQemuImg invokes `qemu-img` binary to inspect the disk size and format.
func inspectDiskWithQemuImg(fName string) (size int64, format string, _ error) {
	info, err := imgutil.GetInfo(fName)
	if err != nil {
		return -1, "", err
	}
	return info.VSize, info.Format, nil
}

func (d *Disk) Lock(instanceDir string) error {
	inUseBy := filepath.Join(d.Dir, filenames.InUseBy)
	return os.Symlink(instanceDir, inUseBy)
}

func (d *Disk) Unlock() error {
	inUseBy := filepath.Join(d.Dir, filenames.InUseBy)
	return os.Remove(inUseBy)
}
