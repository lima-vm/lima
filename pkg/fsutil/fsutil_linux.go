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
