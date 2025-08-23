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

	"github.com/lima-vm/lima/v2/pkg/limatype/filenames"
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

// inspectDisk attempts to inspect the disk size and format with qcow2reader.
func inspectDisk(fName string) (size int64, format string, _ error) {
	f, err := os.Open(fName)
	if err != nil {
		return -1, "", err
	}
	defer f.Close()
	img, err := qcow2reader.Open(f)
	if err != nil {
		return -1, "", err
	}
	sz := img.Size()
	if sz < 0 {
		return -1, "", fmt.Errorf("cannot determine size of %q", fName)
	}

	return sz, string(img.Type()), nil
}

func (d *Disk) Lock(instanceDir string) error {
	inUseBy := filepath.Join(d.Dir, filenames.InUseBy)
	return os.Symlink(instanceDir, inUseBy)
}

func (d *Disk) Unlock() error {
	inUseBy := filepath.Join(d.Dir, filenames.InUseBy)
	return os.Remove(inUseBy)
}
