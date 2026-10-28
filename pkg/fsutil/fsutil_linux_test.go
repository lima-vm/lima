// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package fsutil

import (
	"os"
	"testing"

	"gotest.tools/v3/assert"
)

// skipUnlessBtrfs skips the test when dir is not on btrfs, where there is nothing to observe.
func skipUnlessBtrfs(t *testing.T, dir string) {
	t.Helper()
	ok, err := onBtrfs(dir)
	assert.NilError(t, err)
	if !ok {
		t.Skip("TMPDIR is not on btrfs")
	}
}

// newFileHasNoCOW reports whether a file created in dir inherits the No_COW attribute.
func newFileHasNoCOW(t *testing.T, dir string) bool {
	t.Helper()
	f, err := os.CreateTemp(dir, "disk")
	assert.NilError(t, err)
	assert.NilError(t, f.Close())
	noCOW, err := hasNoCOW(f.Name())
	assert.NilError(t, err)
	return noCOW
}

// TestDisableCopyOnWrite covers the btrfs case only when TMPDIR is on btrfs.
func TestDisableCopyOnWrite(t *testing.T) {
	dir := t.TempDir()
	assert.NilError(t, DisableCopyOnWrite(dir))
	skipUnlessBtrfs(t, dir)
	assert.Assert(t, newFileHasNoCOW(t, dir), "a file created in dir should inherit No_COW")
}

// TestDisableCopyOnWriteEnv covers the btrfs case only when TMPDIR is on btrfs.
func TestDisableCopyOnWriteEnv(t *testing.T) {
	for value, noCOW := range map[string]bool{
		"false": false,
		"0":     false,
		"true":  true,
		"bogus": true, // an invalid value is ignored
	} {
		t.Run(value, func(t *testing.T) {
			t.Setenv("LIMA_BTRFS_NOCOW", value)
			dir := t.TempDir()
			assert.NilError(t, setNoCOW(dir, false)) // whatever TMPDIR is
			assert.NilError(t, DisableCopyOnWrite(dir))
			skipUnlessBtrfs(t, dir)
			assert.Equal(t, newFileHasNoCOW(t, dir), noCOW)
		})
	}
}

// TestMatchCopyOnWrite covers the btrfs case only when TMPDIR is on btrfs.
func TestMatchCopyOnWrite(t *testing.T) {
	cow, noCOW, dst := t.TempDir(), t.TempDir(), t.TempDir()
	assert.NilError(t, MatchCopyOnWrite(dst, cow))
	skipUnlessBtrfs(t, dst)
	assert.NilError(t, setNoCOW(cow, false)) // whatever TMPDIR is
	assert.NilError(t, setNoCOW(noCOW, true))
	// The opt-out is for new instances; a copy of an existing one still follows it.
	t.Setenv("LIMA_BTRFS_NOCOW", "false")

	assert.NilError(t, MatchCopyOnWrite(dst, noCOW))
	assert.Assert(t, newFileHasNoCOW(t, dst), "dst should follow a No_COW src")
	assert.NilError(t, MatchCopyOnWrite(dst, cow))
	assert.Assert(t, !newFileHasNoCOW(t, dst), "dst should follow a copy-on-write src")
}
