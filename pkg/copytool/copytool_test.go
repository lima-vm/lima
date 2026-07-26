// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package copytool

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"gotest.tools/v3/assert"

	"github.com/lima-vm/lima/v2/pkg/limatype"
	"github.com/lima-vm/lima/v2/pkg/limatype/dirnames"
	"github.com/lima-vm/lima/v2/pkg/limatype/filenames"
	"github.com/lima-vm/lima/v2/pkg/sshutil"
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

// TestParseCopyPathsWindowsDriveLetter locks in the split between Windows
// absolute paths, which are local, and drive-relative paths like "C:foo.txt",
// which must stay instance "C" so single-letter instance names keep working.
func TestParseCopyPathsWindowsDriveLetter(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows-only path handling")
	}
	t.Setenv("LIMA_HOME", t.TempDir())
	ctx := t.Context()

	// Absolute drive-letter paths are local, and keep the native form the
	// backends convert for their own tool.
	for _, p := range []string{`C:\foo`, `C:/foo`} {
		cps, err := parseCopyPaths(ctx, []string{p})
		assert.NilError(t, err)
		assert.Equal(t, len(cps), 1)
		assert.Equal(t, cps[0].IsRemote, false, "%q must be a local path", p)
		assert.Equal(t, cps[0].Path, p, "%q must reach the backend unconverted", p)
	}

	// "C:foo" is drive-relative (not absolute). It must reach the
	// instance-lookup branch as instance "C" path "foo" and fail at
	// store.Inspect with an instance-not-found error.
	_, err := parseCopyPaths(ctx, []string{"C:foo"})
	assert.ErrorContains(t, err, "instance `C`")
	assert.ErrorContains(t, err, "does not exist")

	// Explicit instance:path behaves the same.
	_, err = parseCopyPaths(ctx, []string{"nonexistent-instance-for-test:/tmp/x"})
	assert.ErrorContains(t, err, "instance `nonexistent-instance-for-test`")
	assert.ErrorContains(t, err, "does not exist")
}

// looksRemoteToRsync reports whether rsync would read arg as host:path, which it
// does for any colon before the first slash.
func looksRemoteToRsync(arg string) bool {
	colon := strings.Index(arg, ":")
	slash := strings.Index(arg, "/")
	return colon >= 0 && (slash < 0 || colon < slash)
}

// TestRsyncCommandLocalPathForm locks in that local operands reach rsync as
// paths. Windows has no native rsync, so a drive letter would be read as a
// hostspec and the transfer would fail.
func TestRsyncCommandLocalPathForm(t *testing.T) {
	// Nothing beside this path, so the conversion takes its in-process
	// fallback and the expected operand holds with or without a Cygwin install.
	tool := &rsyncTool{toolPath: filepath.Join(t.TempDir(), "rsync"), Options: &Options{}}

	src, dst := "/tmp/src", "/tmp/dst"
	wantSrc, wantDst := src, dst
	if runtime.GOOS == "windows" {
		src, dst = `C:\src`, `C:\dst`
		wantSrc, wantDst = "/c/src", "/c/dst"
	}

	cmd, err := tool.Command(t.Context(), []string{src, dst}, nil)
	assert.NilError(t, err)

	operands := cmd.Args[len(cmd.Args)-2:]
	assert.Equal(t, operands[0], wantSrc)
	assert.Equal(t, operands[1], wantDst)
	for _, arg := range operands {
		assert.Assert(t, !looksRemoteToRsync(arg), "rsync reads %q as host:path", arg)
	}
}

// TestSCPCommandLocalPathForm locks in that local operands reach scp in the
// form its own toolchain reads. Native Windows OpenSSH takes a drive letter,
// which a Cygwin scp would read as a hostspec instead.
func TestSCPCommandLocalPathForm(t *testing.T) {
	limaHome := t.TempDir()
	t.Setenv("LIMA_HOME", limaHome)
	// CommonOpts requires the internal identity to exist.
	configDir := filepath.Join(limaHome, "_config")
	assert.NilError(t, os.MkdirAll(configDir, 0o700))
	assert.NilError(t, os.WriteFile(filepath.Join(configDir, filenames.UserPrivateKey), nil, 0o600))

	// Nothing beside this path, so the conversion takes its native fallback and
	// the expected operand holds with or without a Cygwin install.
	tool := &scpTool{
		toolPath: filepath.Join(t.TempDir(), "scp"),
		sshExe:   sshutil.SSHExe{Exe: "ssh"},
		Options:  &Options{},
	}

	src, dst := "/tmp/src", "/tmp/dst"
	wantSrc, wantDst := src, dst
	if runtime.GOOS == "windows" {
		src, dst = `C:\src`, `C:\dst`
		wantSrc, wantDst = "C:/src", "C:/dst"
	}

	// Two local operands, so no instance lookup and no host-specific options.
	cmd, err := tool.Command(t.Context(), []string{src, dst}, nil)
	assert.NilError(t, err)

	operands := cmd.Args[len(cmd.Args)-2:]
	assert.Equal(t, operands[0], wantSrc)
	assert.Equal(t, operands[1], wantDst)

	// scp hands IdentityFile to the ssh beside itself, so it must carry scp's
	// path form, which is not always the form of the ssh Lima selected.
	var identityFile string
	for _, arg := range cmd.Args {
		if strings.HasPrefix(arg, "IdentityFile=") {
			identityFile = arg
		}
	}
	assert.Assert(t, identityFile != "", "no IdentityFile option: %v", cmd.Args)
	if runtime.GOOS == "windows" {
		// Pin the whole value: the Cygwin form carries no backslash either, so
		// a laxer check would pass even if the form followed sshExe again.
		// LimaConfigDir resolves symlinks, which on Windows runners also expands
		// the 8.3 temp path, so build the expectation from the same helper.
		resolvedConfigDir, err := dirnames.LimaConfigDir()
		assert.NilError(t, err)
		want := "IdentityFile='" + filepath.ToSlash(filepath.Join(resolvedConfigDir, filenames.UserPrivateKey)) + "'"
		assert.Equal(t, identityFile, want)
	}
}

// TestSSHOptsForInstanceMultiplexing locks in the multiplexing policy shared by
// the rsync availability probe and the rsync and scp commands.
func TestSSHOptsForInstanceMultiplexing(t *testing.T) {
	limaHome := t.TempDir()
	t.Setenv("LIMA_HOME", limaHome)
	// CommonOpts requires the internal identity to exist.
	configDir := filepath.Join(limaHome, "_config")
	assert.NilError(t, os.MkdirAll(configDir, 0o700))
	assert.NilError(t, os.WriteFile(filepath.Join(configDir, filenames.UserPrivateKey), nil, 0o600))

	name := "test-instance"
	inst := &limatype.Instance{
		// Only the control socket path derives from Dir, and nothing opens it.
		// Keep it short: SSHOpts rejects a socket path over UNIX_PATH_MAX, which
		// the temp directory on macOS exceeds on its own.
		Dir:    filepath.Join("/tmp", name),
		Config: &limatype.LimaYAML{User: limatype.User{Name: &name}},
	}

	opts, err := sshOptsForInstance(t.Context(), sshutil.SSHExe{Exe: "ssh"}, "ssh", inst)
	assert.NilError(t, err)
	assert.Assert(t, slices.Contains(opts, "User="+name), "opts: %v", opts)

	var mux []string
	for _, o := range opts {
		if strings.HasPrefix(o, "ControlMaster") || strings.HasPrefix(o, "ControlPath") || strings.HasPrefix(o, "ControlPersist") {
			mux = append(mux, o)
		}
	}
	if runtime.GOOS == "windows" {
		// Native OpenSSH cannot multiplex, and Cygwin ssh's is unreliable.
		assert.Equal(t, len(mux), 0, "Windows must not multiplex: %v", mux)
	} else {
		assert.Equal(t, len(mux), 3, "other platforms multiplex: %v", opts)
	}
}

// TestNewAutoSurfacesPathError verifies that the default backend reports a bad
// path as itself, rather than as a missing copy tool.
func TestNewAutoSurfacesPathError(t *testing.T) {
	t.Setenv("LIMA_HOME", t.TempDir())
	paths := []string{"nonexistent-instance-for-test:/tmp/x", "/tmp/y"}

	_, err := New(t.Context(), string(BackendAuto), paths, &Options{})
	assert.ErrorContains(t, err, "instance `nonexistent-instance-for-test`")
}
