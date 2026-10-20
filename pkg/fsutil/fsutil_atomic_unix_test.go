//go:build unix

// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package fsutil

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"gotest.tools/v3/assert"
)

// TestWriteFileAtomicModeUnderUmask records that the 0600 os.CreateTemp requests is masked by the
// process umask. umask is process-global, so the test restores it and must not run in parallel.
func TestWriteFileAtomicModeUnderUmask(t *testing.T) {
	saved := syscall.Umask(0)
	t.Cleanup(func() { syscall.Umask(saved) })
	for _, tc := range []struct {
		umask int
		want  os.FileMode
	}{
		{0o022, 0o600},
		{0o077, 0o600},
		{0o400, 0o200},
		{0o200, 0o400},
	} {
		// Create the dir under umask 0 (accessible for Stat and cleanup); apply the test umask only
		// across the write itself.
		dir := t.TempDir()
		path := filepath.Join(dir, "marker")
		syscall.Umask(tc.umask)
		err := WriteFileAtomic(path, []byte("x"))
		syscall.Umask(0)
		assert.NilError(t, err)
		fi, err := os.Stat(path)
		assert.NilError(t, err)
		assert.Equal(t, fi.Mode().Perm(), tc.want, "umask %#o", tc.umask)
	}
}

// TestWriteFileAtomicLongBasename checks that the temp file uses a short fixed prefix: a long but
// valid destination basename must not overflow the name limit through a basename-derived temp name.
func TestWriteFileAtomicLongBasename(t *testing.T) {
	path := filepath.Join(t.TempDir(), strings.Repeat("a", 250))
	assert.NilError(t, WriteFileAtomic(path, []byte("x")))
	got, err := os.ReadFile(path)
	assert.NilError(t, err)
	assert.Equal(t, string(got), "x")
}

// TestWriteFileAtomicOverwriteResetsMode records that overwriting an existing file gives it the mode
// os.CreateTemp requests masked by the umask, rather than preserving the old file's permission bits.
// It mirrors the new-file table against a pre-existing 0666 destination. umask is process-global, so
// the test restores it and must not run in parallel.
func TestWriteFileAtomicOverwriteResetsMode(t *testing.T) {
	saved := syscall.Umask(0)
	t.Cleanup(func() { syscall.Umask(saved) })
	for _, tc := range []struct {
		umask int
		want  os.FileMode
	}{
		{0o022, 0o600},
		{0o077, 0o600},
		{0o400, 0o200},
		{0o200, 0o400},
	} {
		// Pre-create a 0666 destination under umask 0 so the broad bits stick, then overwrite it
		// under the test umask. A rewrite that kept the old mode would leave 0666, not tc.want.
		path := filepath.Join(t.TempDir(), "marker")
		assert.NilError(t, os.WriteFile(path, []byte("old"), 0o666))
		syscall.Umask(tc.umask)
		err := WriteFileAtomic(path, []byte("new"))
		syscall.Umask(0)
		assert.NilError(t, err)
		fi, err := os.Stat(path)
		assert.NilError(t, err)
		assert.Equal(t, fi.Mode().Perm(), tc.want, "umask %#o", tc.umask)
	}
}
