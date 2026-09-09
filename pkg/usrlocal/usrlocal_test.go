// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package usrlocal

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"gotest.tools/v3/assert"
)

func setExecutableViaArgs0(t *testing.T, executable string) {
	t.Helper()
	original := ExecutableViaArgs0
	ExecutableViaArgs0 = func() (string, error) {
		return executable, nil
	}
	t.Cleanup(func() {
		ExecutableViaArgs0 = original
	})
}

func TestReadFileFromDirs(t *testing.T) {
	firstDir := t.TempDir()
	secondDir := t.TempDir()
	name := "defaults/example.yaml"

	secondPath := filepath.Join(secondDir, filepath.FromSlash(name))
	assert.NilError(t, os.MkdirAll(filepath.Dir(secondPath), 0o755))
	assert.NilError(t, os.WriteFile(secondPath, []byte("second"), 0o644))

	b, err := readFileFromDirs(name, []string{firstDir, secondDir})
	assert.NilError(t, err)
	assert.Equal(t, string(b), "second")

	firstPath := filepath.Join(firstDir, filepath.FromSlash(name))
	assert.NilError(t, os.MkdirAll(filepath.Dir(firstPath), 0o755))
	assert.NilError(t, os.WriteFile(firstPath, []byte("first"), 0o644))

	b, err = readFileFromDirs(name, []string{firstDir, secondDir})
	assert.NilError(t, err)
	assert.Equal(t, string(b), "first")
}

func TestReadFileFromDirsNotFound(t *testing.T) {
	_, err := readFileFromDirs("missing", []string{t.TempDir()})
	assert.Assert(t, errors.Is(err, fs.ErrNotExist))
}

func TestReadFile(t *testing.T) {
	prefix := t.TempDir()
	path := filepath.Join(prefix, "share", "lima", "defaults", "example.yaml")
	assert.NilError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	assert.NilError(t, os.WriteFile(path, []byte("example"), 0o644))
	setExecutableViaArgs0(t, filepath.Join(prefix, "bin", "limactl"))

	b, err := ReadFile("defaults/example.yaml")
	assert.NilError(t, err)
	assert.Equal(t, string(b), "example")
}

func TestReadFileDoesNotUseSourceTree(t *testing.T) {
	setExecutableViaArgs0(t, filepath.Join(t.TempDir(), "bin", "limactl"))

	_, err := ReadFile("defaults/containerd.yaml")
	assert.Assert(t, errors.Is(err, fs.ErrNotExist))
}

func TestReadFileRejectsInvalidName(t *testing.T) {
	_, err := ReadFile("../containerd.yaml")
	assert.ErrorContains(t, err, "invalid resource path")
}
