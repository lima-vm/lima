// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package editorcmd

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"gotest.tools/v3/assert"
)

func TestDetectWindowsPath(t *testing.T) {
	editorPath := makeWindowsEditor(t)
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", editorPath)

	path, args := Detect()

	assert.Equal(t, path, editorPath)
	assert.Equal(t, len(args), 0)
}

func TestDetectWindowsPathWithArguments(t *testing.T) {
	editorPath := makeWindowsEditor(t)
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", fmt.Sprintf(`'%s' --wait`, editorPath))

	path, args := Detect()

	assert.Equal(t, path, editorPath)
	assert.DeepEqual(t, args, []string{"--wait"})
}

func makeWindowsEditor(t *testing.T) string {
	t.Helper()

	editorDir := filepath.Join(t.TempDir(), "editor with spaces")
	assert.NilError(t, os.Mkdir(editorDir, 0o755))
	editorPath := filepath.Join(editorDir, "editor.exe")
	assert.NilError(t, os.WriteFile(editorPath, nil, 0o755))
	return editorPath
}
