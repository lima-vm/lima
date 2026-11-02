//go:build unix

// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"gotest.tools/v3/assert"

	"github.com/lima-vm/lima/v2/pkg/store"
)

// TestDiskImportCopyFailure checks that a failed copy fails the import and removes the disk
// directory that it created. Under umask 0o222 the import creates a read-only disk directory,
// and the copy into it fails. umask is process-global, so the test must not run in parallel.
func TestDiskImportCopyFailure(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root can write into a read-only directory")
	}
	t.Setenv("LIMA_HOME", t.TempDir())
	diskDir, err := store.DiskDir("imported")
	assert.NilError(t, err)
	assert.NilError(t, os.MkdirAll(filepath.Dir(diskDir), 0o755))
	source := filepath.Join(t.TempDir(), "source.raw")
	assert.NilError(t, os.WriteFile(source, make([]byte, 4096), 0o644))

	saved := syscall.Umask(0o222)
	err = diskImportAction(nil, []string{"imported", source})
	syscall.Umask(saved)

	assert.ErrorIs(t, err, fs.ErrPermission)
	_, err = os.Stat(diskDir)
	assert.ErrorIs(t, err, fs.ErrNotExist)
}
