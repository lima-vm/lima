// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package fsutil

import (
	"os"
	"path/filepath"
)

// WriteFileAtomic writes data to a temp file in path's directory and renames it onto path, so a reader
// sees the old file or the whole new one, never a partial write; the rename is atomic on Unix only.
// os.CreateTemp requests 0600, so the result is 0600 masked by the umask (0600 under a typical umask).
func WriteFileAtomic(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".lima-atomic-*.tmp")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer func() { _ = os.Remove(name) }()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(name, path)
}
