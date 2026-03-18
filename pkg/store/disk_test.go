// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package store

import (
	"os"
	"path/filepath"
	"testing"

	"gotest.tools/v3/assert"

	"github.com/lima-vm/lima/v2/pkg/limatype/filenames"
)

func TestDiskLockForInstance(t *testing.T) {
	t.Run("unlocked disk", func(t *testing.T) {
		diskDir := t.TempDir()
		instanceDir := t.TempDir()
		disk := &Disk{Name: "testdisk", Dir: diskDir}

		assert.NilError(t, disk.LockForInstance(instanceDir))
		target, err := os.Readlink(filepath.Join(diskDir, filenames.InUseBy))
		assert.NilError(t, err)
		assert.Equal(t, target, instanceDir)
	})

	t.Run("locked by same instance (stale lock)", func(t *testing.T) {
		diskDir := t.TempDir()
		instanceDir := t.TempDir()
		disk := &Disk{
			Name:        "testdisk",
			Dir:         diskDir,
			Instance:    filepath.Base(instanceDir),
			InstanceDir: instanceDir,
		}
		// Simulate stale lock from previous run
		assert.NilError(t, os.Symlink(instanceDir, filepath.Join(diskDir, filenames.InUseBy)))

		// LockForInstance should auto-unlock and re-lock
		assert.NilError(t, disk.LockForInstance(instanceDir))
		target, err := os.Readlink(filepath.Join(diskDir, filenames.InUseBy))
		assert.NilError(t, err)
		assert.Equal(t, target, instanceDir)
	})

	t.Run("locked by different instance", func(t *testing.T) {
		diskDir := t.TempDir()
		otherDir := t.TempDir()
		disk := &Disk{
			Name:        "testdisk",
			Dir:         diskDir,
			Instance:    filepath.Base(otherDir),
			InstanceDir: otherDir,
		}
		assert.NilError(t, os.Symlink(otherDir, filepath.Join(diskDir, filenames.InUseBy)))

		newInstanceDir := t.TempDir()
		err := disk.LockForInstance(newInstanceDir)
		assert.ErrorContains(t, err, "in use by instance")
	})
}
