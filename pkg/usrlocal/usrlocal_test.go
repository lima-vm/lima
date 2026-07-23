// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package usrlocal

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gotest.tools/v3/assert"
)

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

func TestReadFileFromSourceTree(t *testing.T) {
	b, err := ReadFile("defaults/containerd.yaml")
	assert.NilError(t, err)
	assert.Assert(t, strings.Contains(string(b), "archives:"))
}

func TestReadFileRejectsInvalidName(t *testing.T) {
	_, err := ReadFile("../containerd.yaml")
	assert.ErrorContains(t, err, "invalid resource path")
}
