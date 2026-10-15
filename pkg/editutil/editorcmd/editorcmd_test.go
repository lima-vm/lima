// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package editorcmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"gotest.tools/v3/assert"
)

// fakeEditor creates an executable named name in dir, which appends its arguments
// (one per line) to dir/args.txt.
func fakeEditor(t *testing.T, dir, name string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	script := "#!/bin/sh\nfor a in \"$@\"; do echo \"$a\" >> \"" + filepath.Join(dir, "args.txt") + "\"; done\n"
	assert.NilError(t, os.WriteFile(p, []byte(script), 0o755))
	return p
}

func setupPath(t *testing.T) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("uses shell scripts as fake editors")
	}
	dir := t.TempDir()
	t.Setenv("PATH", dir)
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", "")
	return dir
}

func TestDetectPlain(t *testing.T) {
	dir := setupPath(t)
	ed := fakeEditor(t, dir, "myeditor")
	t.Setenv("EDITOR", "myeditor")
	assert.DeepEqual(t, Detect(), []string{ed})
}

func TestDetectPathWithSpace(t *testing.T) {
	dir := setupPath(t)
	sub := filepath.Join(dir, "my editors")
	assert.NilError(t, os.Mkdir(sub, 0o755))
	ed := fakeEditor(t, sub, "myeditor")
	t.Setenv("EDITOR", ed)
	assert.DeepEqual(t, Detect(), []string{ed})
}

func TestDetectWithFlags(t *testing.T) {
	dir := setupPath(t)
	ed := fakeEditor(t, dir, "myeditor")
	t.Setenv("VISUAL", "myeditor -w --flag")
	assert.DeepEqual(t, Detect(), []string{ed, "-w", "--flag"})
}

func TestDetectFallsThrough(t *testing.T) {
	dir := setupPath(t)
	ed := fakeEditor(t, dir, "vi")
	t.Setenv("VISUAL", "nonexistent -w")
	assert.DeepEqual(t, Detect(), []string{ed})
}

func TestDetectNone(t *testing.T) {
	setupPath(t)
	assert.Assert(t, Detect() == nil)
}

func TestDetectShellQuoting(t *testing.T) {
	dir := setupPath(t)
	fakeEditor(t, dir, "myeditor")
	t.Setenv("EDITOR", `myeditor --title "a b"`)
	editor := Detect()
	assert.Equal(t, editor[0], "/bin/sh")

	file := filepath.Join(dir, "file with space.yaml")
	cmd := exec.CommandContext(t.Context(), editor[0], append(editor[1:], file)...)
	cmd.Env = append(os.Environ(), "PATH="+dir+":/usr/bin:/bin")
	assert.NilError(t, cmd.Run())
	b, err := os.ReadFile(filepath.Join(dir, "args.txt"))
	assert.NilError(t, err)
	assert.DeepEqual(t, strings.Split(strings.TrimSuffix(string(b), "\n"), "\n"), []string{"--title", "a b", file})
}
