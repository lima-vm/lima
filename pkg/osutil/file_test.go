// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package osutil

import (
	"net"
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

func TestRemoveStaleSocket(t *testing.T) {
	base := t.TempDir()

	// A non-existent path is not an error.
	assert.NilError(t, RemoveStaleSocket(filepath.Join(base, "absent")))

	// A regular file at the socket path is left in place, not deleted.
	file := filepath.Join(base, "file")
	assert.NilError(t, os.WriteFile(file, []byte("keep\n"), 0o600))
	assert.Assert(t, RemoveStaleSocket(file) != nil, "expected a regular file to be refused")
	assert.Assert(t, FileExists(file), "regular file must not be removed")

	// A directory at the socket path is left in place, not deleted recursively.
	dir := filepath.Join(base, "dir")
	assert.NilError(t, os.MkdirAll(dir, 0o700))
	assert.NilError(t, os.WriteFile(filepath.Join(dir, "child"), []byte("keep\n"), 0o600))
	assert.Assert(t, RemoveStaleSocket(dir) != nil, "expected a directory to be refused")
	assert.Assert(t, FileExists(filepath.Join(dir, "child")), "directory contents must not be removed")

	// An actual stale socket is removed so a new listener can bind.
	sock := filepath.Join(base, "sock")
	var lc net.ListenConfig
	l, err := lc.Listen(t.Context(), "unix", sock)
	assert.NilError(t, err)
	defer l.Close()
	assert.NilError(t, RemoveStaleSocket(sock))
	assert.Assert(t, !FileExists(sock), "stale socket must be removed")
}
