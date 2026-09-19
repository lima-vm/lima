// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package copytool

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"

	"gotest.tools/v3/assert"
)

// TestCommandDoesNotMutateOptions verifies that passing opts to Command() does not
// overwrite the tool's stored Options for subsequent calls.
func TestCommandDoesNotMutateOptions(t *testing.T) {
	initial := &Options{Verbose: false, Recursive: false}
	tool, err := newRsyncTool(initial)
	if err != nil {
		t.Skip("rsync not found:", err)
	}

	override := &Options{Verbose: true, Recursive: true}
	// Use local paths to avoid instance lookup
	_, _ = tool.Command(t.Context(), []string{"/tmp/src", "/tmp/dst"}, override)

	assert.Equal(t, tool.Options.Verbose, false, "Command() must not mutate stored Options.Verbose")
	assert.Equal(t, tool.Options.Recursive, false, "Command() must not mutate stored Options.Recursive")
}

// TestRsyncCommandEndsOptionParsing verifies that a path starting with a dash is
// passed after "--", so rsync cannot mistake it for an option such as --rsh.
func TestRsyncCommandEndsOptionParsing(t *testing.T) {
	tool, err := newRsyncTool(&Options{})
	if err != nil {
		t.Skip("rsync not found:", err)
	}

	const dashPath = "--rsh=touch pwned"
	// Use local paths to avoid instance lookup
	cmd, err := tool.Command(t.Context(), []string{dashPath, "/tmp/dst"}, nil)
	assert.NilError(t, err)

	sep := slices.Index(cmd.Args, "--")
	assert.Assert(t, sep != -1, "rsync args must contain the %#q separator: %v", "--", cmd.Args)
	assert.Assert(t, slices.Index(cmd.Args, dashPath) > sep, "path %#q must come after %#q: %v", dashPath, "--", cmd.Args)
}

// TestRsyncCommandPaths verifies that the paths of a recursive copy are passed to rsync in
// the form that makes it behave like `cp -r` and `scp -r`, regardless of how the trailing
// slashes were spelled by the caller.
// https://github.com/lima-vm/lima/issues/4468
// https://github.com/lima-vm/lima/issues/5500
func TestRsyncCommandPaths(t *testing.T) {
	if _, err := newRsyncTool(&Options{}); err != nil {
		t.Skip("rsync not found:", err)
	}

	// Only local paths are used, so that no guest is involved.
	tmpDir := t.TempDir()
	dir := filepath.Join(tmpDir, "dir")
	assert.NilError(t, os.Mkdir(dir, 0o755))
	dir2 := filepath.Join(tmpDir, "dir2")
	assert.NilError(t, os.Mkdir(dir2, 0o755))
	file := filepath.Join(tmpDir, "file.txt")
	assert.NilError(t, os.WriteFile(file, []byte("hello"), 0o644))
	existing := filepath.Join(tmpDir, "existing")
	assert.NilError(t, os.Mkdir(existing, 0o755))
	missing := filepath.Join(tmpDir, "missing")

	tests := []struct {
		name     string
		opts     Options
		args     []string
		expected []string
	}{
		{
			// The destination becomes a copy of the source directory.
			name: "dir into missing dst", opts: Options{Recursive: true},
			args: []string{dir, missing}, expected: []string{dir + "/", missing},
		},
		{
			// The source directory is copied into the destination.
			name: "dir into existing dst", opts: Options{Recursive: true},
			args: []string{dir, existing}, expected: []string{dir, existing},
		},
		{
			// A trailing slash on the source is not significant.
			name: "dir with slash into existing dst", opts: Options{Recursive: true},
			args: []string{dir + "/", existing}, expected: []string{dir, existing},
		},
		{
			name: "dir with slash into missing dst", opts: Options{Recursive: true},
			args: []string{dir + "/", missing}, expected: []string{dir + "/", missing},
		},
		{
			// A trailing slash would just make rsync fail on a non-directory.
			name: "file into missing dst", opts: Options{Recursive: true},
			args: []string{file, missing}, expected: []string{file, missing},
		},
		{
			// Multiple sources are never merged into the destination.
			name: "multiple sources", opts: Options{Recursive: true},
			args: []string{dir + "/", dir2, missing}, expected: []string{dir, dir2, missing},
		},
		{
			// Without -r the paths reach rsync as they are, trailing slash included.
			// `limactl shell --sync` relies on it to mirror the directories.
			name: "non-recursive", opts: Options{},
			args: []string{dir + "/", existing}, expected: []string{dir + "/", existing},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tool, err := newRsyncTool(&tt.opts)
			assert.NilError(t, err)
			cmd, err := tool.Command(t.Context(), tt.args, nil)
			assert.NilError(t, err)

			// The paths are everything after the "--" separator.
			sep := slices.Index(cmd.Args, "--")
			assert.Assert(t, sep != -1, "rsync args must contain the %#q separator: %v", "--", cmd.Args)
			assert.DeepEqual(t, cmd.Args[sep+1:], tt.expected)
		})
	}
}

// TestRsyncCommandUndeterminedPath verifies that a path that can neither be confirmed nor
// ruled out to be a directory fails the copy, rather than falling back to copying the
// contents of the source, which would put the files in the wrong place.
func TestRsyncCommandUndeterminedPath(t *testing.T) {
	if runtime.GOOS == "windows" {
		// The test needs a path that can neither be confirmed nor ruled out to be a
		// directory. A file used as a directory component fails with ENOTDIR on POSIX
		// systems, but Windows reports it as a missing path instead, which os.Stat
		// turns into fs.ErrNotExist. See the comment in $GOROOT/src/os/root_windows.go.
		t.Skip("ENOTDIR for a file used as a directory is POSIX-specific")
	}
	tool, err := newRsyncTool(&Options{Recursive: true})
	if err != nil {
		t.Skip("rsync not found:", err)
	}

	tmpDir := t.TempDir()
	dir := filepath.Join(tmpDir, "dir")
	assert.NilError(t, os.Mkdir(dir, 0o755))
	file := filepath.Join(tmpDir, "file.txt")
	assert.NilError(t, os.WriteFile(file, []byte("hello"), 0o644))

	// Stat fails with ENOTDIR, rather than reporting the destination as missing.
	_, err = tool.Command(t.Context(), []string{dir, filepath.Join(file, "dst")}, nil)
	assert.ErrorContains(t, err, "not a directory")
}

func TestTrimTrailingSlashes(t *testing.T) {
	tests := []struct{ path, expected string }{
		{"/tmp/foo", "/tmp/foo"},
		{"/tmp/foo/", "/tmp/foo"},
		{"/tmp/foo//", "/tmp/foo"},
		{"/", "/"},
		{"", ""},
	}
	for _, tt := range tests {
		assert.Equal(t, trimTrailingSlashes(tt.path), tt.expected, "path=%q", tt.path)
	}
}

func TestQuoteRemotePath(t *testing.T) {
	tests := []struct{ path, expected string }{
		{"/tmp/foo", "/tmp/foo"},
		{"foo bar", "'foo bar'"},
		{"/tmp/$(touch pwned)", `'/tmp/$(touch pwned)'`},
		// A leading "~" or "~user" has to stay expandable by the shell of the guest.
		{"~", "~"},
		{"~/foo bar", "~/'foo bar'"},
		{"~foo", "~foo"},
		{"~foo/bar", "~foo/bar"},
		{"~foo/bar baz", "~foo/'bar baz'"},
		{"~$(touch pwned)/foo", `'~$(touch pwned)/foo'`},
	}
	for _, tt := range tests {
		assert.Equal(t, quoteRemotePath(tt.path), tt.expected, "path=%q", tt.path)
	}
}

// TestNewAutoSurfacesPathError verifies that the default backend reports a bad
// path as itself, rather than as a missing copy tool.
func TestNewAutoSurfacesPathError(t *testing.T) {
	t.Setenv("LIMA_HOME", t.TempDir())
	paths := []string{"nonexistent-instance-for-test:/tmp/x", "/tmp/y"}

	_, err := New(t.Context(), string(BackendAuto), paths, &Options{})
	assert.ErrorContains(t, err, "instance `nonexistent-instance-for-test` does not exist")
}
