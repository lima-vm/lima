// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package fsutil

import (
	"os"
	"path/filepath"
	"testing"

	"gotest.tools/v3/assert"
)

func TestWriteFileAtomic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "marker")

	// A new file is created with the given content.
	assert.NilError(t, WriteFileAtomic(path, []byte("first\n")))
	got, err := os.ReadFile(path)
	assert.NilError(t, err)
	assert.Equal(t, string(got), "first\n")

	// Rewriting replaces the content in place and leaves no temporary file behind.
	assert.NilError(t, WriteFileAtomic(path, []byte("second\n")))
	got, err = os.ReadFile(path)
	assert.NilError(t, err)
	assert.Equal(t, string(got), "second\n")

	entries, err := os.ReadDir(dir)
	assert.NilError(t, err)
	assert.Equal(t, len(entries), 1, "only the destination file should remain, with no leftover temp")
	assert.Equal(t, entries[0].Name(), "marker")
}

func TestWriteFileAtomicErrorLeavesNoTemp(t *testing.T) {
	dir := t.TempDir()
	// Make the parent of the destination a regular file, so creating the temp fails immediately;
	// WriteFileAtomic must surface the error and not leave a temp file lying around.
	notDir := filepath.Join(dir, "file")
	assert.NilError(t, os.WriteFile(notDir, nil, 0o644))

	err := WriteFileAtomic(filepath.Join(notDir, "child"), []byte("x"))
	assert.Assert(t, err != nil, "writing under a non-directory should fail")

	entries, err := os.ReadDir(dir)
	assert.NilError(t, err)
	assert.Equal(t, len(entries), 1, "no temp file should be created under the destination's parent")
}

func TestWriteFileAtomicCleansUpAfterRenameFailure(t *testing.T) {
	dir := t.TempDir()
	// The destination is an existing directory, so the temp file is created and written and only the
	// rename fails, exercising the deferred cleanup that the immediate-failure test above never reaches.
	dst := filepath.Join(dir, "dst")
	assert.NilError(t, os.Mkdir(dst, 0o700))

	err := WriteFileAtomic(dst, []byte("x"))
	assert.Assert(t, err != nil, "renaming onto an existing directory should fail")

	entries, err := os.ReadDir(dir)
	assert.NilError(t, err)
	assert.Equal(t, len(entries), 1, "only the destination directory should remain, with no leftover temp")
	assert.Equal(t, entries[0].Name(), "dst")
}
