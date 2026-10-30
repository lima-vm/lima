// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package fsutil

import (
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/unix"
	"gotest.tools/v3/assert"
)

// TestDisableCopyOnWrite covers the btrfs case only when TMPDIR is on btrfs.
func TestDisableCopyOnWrite(t *testing.T) {
	dir := t.TempDir()
	assert.NilError(t, DisableCopyOnWrite(dir))

	var sf unix.Statfs_t
	assert.NilError(t, unix.Statfs(dir, &sf))
	if uint32(sf.Type) != unix.BTRFS_SUPER_MAGIC {
		t.Skip("TMPDIR is not on btrfs")
	}
	f, err := os.Create(filepath.Join(dir, "disk"))
	assert.NilError(t, err)
	defer f.Close()
	flags, err := unix.IoctlGetUint32(int(f.Fd()), unix.FS_IOC_GETFLAGS)
	assert.NilError(t, err)
	assert.Assert(t, flags&fsNoCOWFL != 0, "a file created in dir should inherit No_COW")
}
