// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package instance

import (
	"os"
	"path/filepath"
	"testing"

	"gotest.tools/v3/assert"

	"github.com/lima-vm/lima/v2/pkg/limatype/filenames"
	"github.com/lima-vm/lima/v2/pkg/osutil"
)

func TestRemoveDiskTarget(t *testing.T) {
	t.Run("no disk", func(t *testing.T) {
		instDir := t.TempDir()
		assert.NilError(t, removeDiskTarget(instDir))
	})

	t.Run("regular disk is kept", func(t *testing.T) {
		instDir := t.TempDir()
		diskPath := filepath.Join(instDir, filenames.Disk)
		assert.NilError(t, os.WriteFile(diskPath, []byte("disk"), 0o600))

		assert.NilError(t, removeDiskTarget(instDir))

		assert.Assert(t, osutil.FileExists(diskPath), "a regular disk file must not be removed")
	})

	t.Run("disk symlinked outside of the instance is removed", func(t *testing.T) {
		instDir := t.TempDir()
		realDiskPath := filepath.Join(t.TempDir(), filenames.Disk)
		assert.NilError(t, os.WriteFile(realDiskPath, []byte("disk"), 0o600))
		diskPath := filepath.Join(instDir, filenames.Disk)
		assert.NilError(t, os.Symlink(realDiskPath, diskPath))

		assert.NilError(t, removeDiskTarget(instDir))

		_, err := os.Lstat(realDiskPath)
		assert.Assert(t, os.IsNotExist(err), "the target of the disk symlink must be removed")
		assert.Assert(t, osutil.IsSymlink(diskPath), "the symlink itself is removed by Delete, not here")
	})

	t.Run("relative disk symlink outside of the instance is removed", func(t *testing.T) {
		instDir := t.TempDir()
		realDiskPath := filepath.Join(t.TempDir(), filenames.Disk)
		assert.NilError(t, os.WriteFile(realDiskPath, []byte("disk"), 0o600))
		relTarget, err := filepath.Rel(instDir, realDiskPath)
		assert.NilError(t, err)
		assert.NilError(t, os.Symlink(relTarget, filepath.Join(instDir, filenames.Disk)))

		assert.NilError(t, removeDiskTarget(instDir))

		_, err = os.Lstat(realDiskPath)
		assert.Assert(t, os.IsNotExist(err), "the target of a relative disk symlink must be removed")
	})

	t.Run("disk symlinked inside of the instance is kept", func(t *testing.T) {
		instDir := t.TempDir()
		// Migrated instances have disk -> diffdisk inside the instance directory.
		legacyDiskPath := filepath.Join(instDir, filenames.DiffDiskLegacy)
		assert.NilError(t, os.WriteFile(legacyDiskPath, []byte("disk"), 0o600))
		assert.NilError(t, os.Symlink(filenames.DiffDiskLegacy, filepath.Join(instDir, filenames.Disk)))

		assert.NilError(t, removeDiskTarget(instDir))

		assert.Assert(t, osutil.FileExists(legacyDiskPath),
			"a disk that lives inside the instance directory must not be removed")
	})

	t.Run("dangling disk symlink", func(t *testing.T) {
		instDir := t.TempDir()
		assert.NilError(t, os.Symlink(filepath.Join(t.TempDir(), filenames.Disk), filepath.Join(instDir, filenames.Disk)))

		assert.NilError(t, removeDiskTarget(instDir))
	})
}
