// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package osutil

import (
	"os"
	"path/filepath"
	"testing"

	"gotest.tools/v3/assert"
)

func TestWriteFileBeneathDir(t *testing.T) {
	base := t.TempDir()
	dir := filepath.Join(base, "mnt")
	assert.NilError(t, os.MkdirAll(dir, 0o700))

	// A regular destination is written normally.
	regular := filepath.Join(dir, "config")
	assert.NilError(t, WriteFileBeneathDir(regular, []byte("ok\n"), 0o600))
	got, err := os.ReadFile(regular)
	assert.NilError(t, err)
	assert.Equal(t, string(got), "ok\n")

	// A symlink at the destination pointing outside the parent directory is
	// refused, and the link target is left untouched.
	secret := filepath.Join(base, "authorized_keys")
	assert.NilError(t, os.WriteFile(secret, []byte("ORIGINAL\n"), 0o600))
	linked := filepath.Join(dir, "linked")
	assert.NilError(t, os.Symlink(secret, linked))

	err = WriteFileBeneathDir(linked, []byte("ATTACKER\n"), 0o600)
	assert.Assert(t, err != nil, "expected the symlinked write to be refused")
	got, err = os.ReadFile(secret)
	assert.NilError(t, err)
	assert.Equal(t, string(got), "ORIGINAL\n")
}
