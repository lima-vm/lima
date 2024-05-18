//go:build linux

package fsutil

import (
	"golang.org/x/sys/unix"
)

func IsNFS(path string) (bool, error) {
	var sf unix.Statfs_t
	if err := unix.Statfs(path, &sf); err != nil {
		return false, err
	}
	return sf.Type == unix.NFS_SUPER_MAGIC, nil
}
