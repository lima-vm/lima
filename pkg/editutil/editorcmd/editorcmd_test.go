// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package editorcmd

import (
	"os/exec"
	"testing"

	"gotest.tools/v3/assert"
)

func TestDetectEditorWithArguments(t *testing.T) {
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", "go --wait")

	expectedPath, err := exec.LookPath("go")
	assert.NilError(t, err)

	path, args := Detect()

	assert.Equal(t, path, expectedPath)
	assert.DeepEqual(t, args, []string{"--wait"})
}

func TestDetectQuotedEditor(t *testing.T) {
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", `"go"`)

	expectedPath, err := exec.LookPath("go")
	assert.NilError(t, err)

	path, args := Detect()

	assert.Equal(t, path, expectedPath)
	assert.Equal(t, len(args), 0)
}

func TestDetectQuotedEditorArgument(t *testing.T) {
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", `go -ldflags "-s -w"`)

	expectedPath, err := exec.LookPath("go")
	assert.NilError(t, err)

	path, args := Detect()

	assert.Equal(t, path, expectedPath)
	assert.DeepEqual(t, args, []string{"-ldflags", "-s -w"})
}

func TestDetectSkipsInvalidEditor(t *testing.T) {
	t.Setenv("VISUAL", `go -c "unterminated`)
	t.Setenv("EDITOR", "go --wait")

	expectedPath, err := exec.LookPath("go")
	assert.NilError(t, err)

	path, args := Detect()

	assert.Equal(t, path, expectedPath)
	assert.DeepEqual(t, args, []string{"--wait"})
}

func TestDetectSkipsEmptyEditor(t *testing.T) {
	t.Setenv("VISUAL", "   ")
	t.Setenv("EDITOR", "go --wait")

	expectedPath, err := exec.LookPath("go")
	assert.NilError(t, err)

	path, args := Detect()

	assert.Equal(t, path, expectedPath)
	assert.DeepEqual(t, args, []string{"--wait"})
}

func TestDetectNoEditor(t *testing.T) {
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", "")
	t.Setenv("PATH", t.TempDir())

	path, args := Detect()

	assert.Equal(t, path, "")
	assert.Equal(t, len(args), 0)
}
