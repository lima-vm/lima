//go:build !windows

// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package lockutil

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"gotest.tools/v3/assert"
)

func TestCheckLockedDir(t *testing.T) {
	open := func(t *testing.T) (dir string, dirFile *os.File) {
		dir = filepath.Join(t.TempDir(), "dir")
		assert.NilError(t, os.Mkdir(dir, 0o755))
		dirFile, err := os.Open(dir)
		assert.NilError(t, err)
		t.Cleanup(func() { dirFile.Close() })
		return dir, dirFile
	}

	t.Run("unchanged", func(t *testing.T) {
		dir, dirFile := open(t)
		assert.NilError(t, checkLockedDir(dirFile, dir))
	})

	t.Run("removed", func(t *testing.T) {
		dir, dirFile := open(t)
		assert.NilError(t, os.Remove(dir))
		assert.Assert(t, errors.Is(checkLockedDir(dirFile, dir), ErrDirRemoved))
	})

	t.Run("removed and recreated", func(t *testing.T) {
		dir, dirFile := open(t)
		assert.NilError(t, os.Remove(dir))
		assert.NilError(t, os.Mkdir(dir, 0o755))
		assert.Assert(t, errors.Is(checkLockedDir(dirFile, dir), ErrDirRemoved))
	})
}
