// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package fsutil

import (
	"errors"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

// IsNFS checks if the path is on NFS. If the path does not exist yet, it will walk
// up parent directories until one exists, or it hits '/' or '.'.
// Any other stat errors will cause IsNFS to fail.
func IsNFS(path string) (bool, error) {
	for len(path) > 1 {
		_, err := os.Stat(path)
		if err == nil {
			break
		}
		if !errors.Is(err, os.ErrNotExist) {
			return false, err
		}
		path = filepath.Dir(path)
	}

	var sf unix.Statfs_t
	if err := unix.Statfs(path, &sf); err != nil {
		return false, err
	}
	return sf.Type == unix.NFS_SUPER_MAGIC, nil
}

// fsNoCOWFL is FS_NOCOW_FL from <linux/fs.h>, which golang.org/x/sys/unix does not define.
const fsNoCOWFL = 0x00800000

// DisableCopyOnWrite sets the No_COW attribute (`chattr +C`) on dir when dir is on btrfs,
// like libvirt does for new storage pools. Files created in dir afterwards inherit it, so
// VM disk images there do not fragment over time. Files already in dir are not affected.
// On other filesystems DisableCopyOnWrite does nothing.
func DisableCopyOnWrite(dir string) error {
	var sf unix.Statfs_t
	if err := unix.Statfs(dir, &sf); err != nil {
		return err
	}
	// Statfs_t.Type is 32-bit on some architectures, where the btrfs magic overflows int32.
	if uint32(sf.Type) != unix.BTRFS_SUPER_MAGIC {
		return nil
	}
	f, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer f.Close()
	flags, err := unix.IoctlGetUint32(int(f.Fd()), unix.FS_IOC_GETFLAGS)
	if err != nil {
		return err
	}
	if flags&fsNoCOWFL != 0 {
		return nil
	}
	return unix.IoctlSetPointerInt(int(f.Fd()), unix.FS_IOC_SETFLAGS, int(flags|fsNoCOWFL))
}
