package store

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/lima-vm/lima/pkg/qemu/imgutil"
	"github.com/lima-vm/lima/pkg/store/filenames"
)

type Disk struct {
	Name        string `json:"name"`
	Size        int64  `json:"size"`
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

	info, err := imgutil.GetInfo(dataDisk)
	if err != nil {
		return nil, err
	}
	disk.Size = info.VSize

	instDir, err := os.Readlink(filepath.Join(diskDir, filenames.InUseBy))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			disk.Instance = ""
			disk.InstanceDir = ""
		} else {
			return nil, err
		}
	} else {
		disk.Instance = filepath.Base(instDir)
		disk.InstanceDir = instDir
	}

	disk.MountPoint = fmt.Sprintf("/mnt/lima-%s", diskName)

	return disk, nil
}

func (d *Disk) Lock(instanceDir string) error {
	inUseBy := filepath.Join(d.Dir, filenames.InUseBy)
	return os.Symlink(instanceDir, inUseBy)
}

func (d *Disk) Unlock() error {
	inUseBy := filepath.Join(d.Dir, filenames.InUseBy)
	return os.Remove(inUseBy)
}
