// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package sshutil

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"gotest.tools/v3/assert"
)

const (
	fakeCygpathOutEnv  = "LIMA_TEST_FAKE_CYGPATH_OUT"
	fakeCygpathFailure = "exit-non-zero"
)

// TestMain doubles as a fake cygpath.exe. With fakeCygpathOutEnv set it prints
// that value and exits 0, or exits 1 when the value is fakeCygpathFailure, so a
// test can copy this binary beside an ssh.exe and drive the Cygwin branch. An
// empty value would not work as the failure signal, because Windows cannot
// distinguish an empty environment variable from an unset one.
//
// It answers only the sftp-server probe and rejects any other command line. A
// change to the arguments in SftpServerForSSH then breaks this test instead of
// going unnoticed. A test driving a different cygpath call needs its own arm.
func TestMain(m *testing.M) {
	if out, ok := os.LookupEnv(fakeCygpathOutEnv); ok {
		if want := []string{"-w", "/usr/lib/ssh/sftp-server"}; !slices.Equal(os.Args[1:], want) {
			os.Stderr.WriteString("fake cygpath: got " + strings.Join(os.Args[1:], " ") +
				", want " + strings.Join(want, " ") + "\n")
			os.Exit(2)
		}
		if out == fakeCygpathFailure {
			os.Exit(1)
		}
		os.Stdout.WriteString(out + "\n")
		os.Exit(0)
	}
	os.Exit(m.Run())
}

// TestPickCompleteSSHOnWindows: an ssh.exe missing scp.exe or
// ssh-keygen.exe (MinGit's shape) is skipped for the next complete
// install on PATH.
func TestPickCompleteSSHOnWindows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows-only PATH walk")
	}

	mkDir := func(t *testing.T, exes ...string) string {
		t.Helper()
		dir := resolvedTempDir(t)
		for _, exe := range exes {
			assert.NilError(t, os.WriteFile(filepath.Join(dir, exe), nil, 0o644))
		}
		return dir
	}

	t.Run("complete install on PATH is picked", func(t *testing.T) {
		full := mkDir(t, "ssh.exe", "scp.exe", "ssh-keygen.exe")
		t.Setenv("PATH", full)
		assert.Equal(t, pickCompleteSSHOnWindows(), filepath.Join(full, "ssh.exe"))
	})

	t.Run("incomplete install before complete install is skipped", func(t *testing.T) {
		mingit := mkDir(t, "ssh.exe")
		full := mkDir(t, "ssh.exe", "scp.exe", "ssh-keygen.exe")
		t.Setenv("PATH", mingit+string(os.PathListSeparator)+full)
		assert.Equal(t, pickCompleteSSHOnWindows(), filepath.Join(full, "ssh.exe"))
	})

	t.Run("falls back to native install when nothing on PATH is complete", func(t *testing.T) {
		nativeSSH := filepath.Join(systemRoot(), "System32", "OpenSSH", "ssh.exe")
		if _, err := os.Stat(nativeSSH); err != nil {
			t.Skipf("native OpenSSH not present at %q on this host", nativeSSH)
		}
		mingit := mkDir(t, "ssh.exe")
		t.Setenv("PATH", mingit)
		assert.Equal(t, pickCompleteSSHOnWindows(), nativeSSH)
	})

	t.Run("incomplete native install disqualifies it too", func(t *testing.T) {
		fakeRoot := resolvedTempDir(t)
		nativeDir := filepath.Join(fakeRoot, "System32", "OpenSSH")
		assert.NilError(t, os.MkdirAll(nativeDir, 0o755))
		assert.NilError(t, os.WriteFile(filepath.Join(nativeDir, "ssh.exe"), nil, 0o644))
		t.Setenv("SystemRoot", fakeRoot)
		t.Setenv("PATH", "")

		assert.Equal(t, pickCompleteSSHOnWindows(), "",
			"an ssh.exe without its companions never qualifies, wherever it lives")
	})
}

// TestNewSSHExeFallsBackToPATH: when no directory holds a complete install,
// NewSSHExe stops being selective and takes whatever `ssh` PATH resolves. The
// binary it returns is therefore reachable only through PATH, which is why a
// host check that only proves the files exist cannot show which ssh Lima runs.
func TestNewSSHExeFallsBackToPATH(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows-only PATH walk")
	}

	partial := resolvedTempDir(t)
	sshExe := filepath.Join(partial, "ssh.exe")
	assert.NilError(t, os.WriteFile(sshExe, nil, 0o644))

	t.Setenv("SystemRoot", resolvedTempDir(t))
	t.Setenv("PATH", partial)
	t.Setenv(EnvShellSSH, "")

	assert.Equal(t, pickCompleteSSHOnWindows(), "", "the partial install must not qualify")

	got, err := NewSSHExe()
	assert.NilError(t, err)
	assert.Equal(t, got.Exe, sshExe)
}

// TestCygpathForSSH: an ssh.exe next to cygpath.exe is Cygwin-based and
// resolves the sibling cygpath; one without is native.
func TestCygpathForSSH(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("cygpath detection only runs on Windows")
	}

	t.Run("cygwin", func(t *testing.T) {
		dir := resolvedTempDir(t)
		sshExe := filepath.Join(dir, "ssh.exe")
		cygpathExe := filepath.Join(dir, "cygpath.exe")
		assert.NilError(t, os.WriteFile(sshExe, nil, 0o644))
		assert.NilError(t, os.WriteFile(cygpathExe, nil, 0o644))

		got, ok := cygpathForSSH(SSHExe{Exe: sshExe})
		assert.Equal(t, ok, true, "ssh.exe next to cygpath.exe should be Cygwin")
		assert.Equal(t, got, cygpathExe, "should return the sibling cygpath, not bare 'cygpath'")
	})

	t.Run("native", func(t *testing.T) {
		dir := resolvedTempDir(t)
		sshExe := filepath.Join(dir, "ssh.exe")
		assert.NilError(t, os.WriteFile(sshExe, nil, 0o644))

		got, ok := cygpathForSSH(SSHExe{Exe: sshExe})
		assert.Equal(t, ok, false, "ssh.exe with no sibling cygpath.exe should be native")
		assert.Equal(t, got, "")
	})

	t.Run("empty", func(t *testing.T) {
		got, ok := cygpathForSSH(SSHExe{})
		assert.Equal(t, ok, false)
		assert.Equal(t, got, "")
	})
}

// TestSftpServerForSSH: a native ssh.exe resolves the sibling
// sftp-server.exe, and returns "" when none exists so the caller falls
// back to sshocker's own auto-detection.
func TestSftpServerForSSH(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows-only path handling")
	}
	ctx := t.Context()

	t.Run("native: sibling sftp-server.exe next to ssh.exe", func(t *testing.T) {
		dir := resolvedTempDir(t)
		sshExe := filepath.Join(dir, "ssh.exe")
		sftpExe := filepath.Join(dir, "sftp-server.exe")
		assert.NilError(t, os.WriteFile(sshExe, nil, 0o644))
		assert.NilError(t, os.WriteFile(sftpExe, nil, 0o644))

		got := SftpServerForSSH(ctx, SSHExe{Exe: sshExe})
		assert.Equal(t, got, sftpExe)
	})

	t.Run("native: no sibling sftp-server.exe returns empty", func(t *testing.T) {
		dir := resolvedTempDir(t)
		sshExe := filepath.Join(dir, "ssh.exe")
		assert.NilError(t, os.WriteFile(sshExe, nil, 0o644))

		got := SftpServerForSSH(ctx, SSHExe{Exe: sshExe})
		assert.Equal(t, got, "", "no sftp-server.exe sibling -> caller falls back to sshocker auto-detect")
	})

	t.Run("empty input returns empty", func(t *testing.T) {
		assert.Equal(t, SftpServerForSSH(ctx, SSHExe{}), "")
	})
}

// TestSftpServerForSSHCygwin: the Cygwin branch reports what the sibling
// cygpath resolves, but only once that names an executable. cygpath omits the
// .exe suffix, so the candidate reaches LookPath without one.
func TestSftpServerForSSHCygwin(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows-only path handling")
	}
	ctx := t.Context()

	t.Run("candidate without .exe resolves to the binary beside it", func(t *testing.T) {
		sshExe := cygwinToolchain(t)
		dir := filepath.Dir(sshExe)
		sftpExe := filepath.Join(dir, "sftp-server.exe")
		assert.NilError(t, os.WriteFile(sftpExe, nil, 0o644))
		t.Setenv(fakeCygpathOutEnv, filepath.Join(dir, "sftp-server"))

		assert.Equal(t, SftpServerForSSH(ctx, SSHExe{Exe: sshExe}), sftpExe)
	})

	t.Run("candidate that resolves to nothing returns empty", func(t *testing.T) {
		sshExe := cygwinToolchain(t)
		t.Setenv(fakeCygpathOutEnv, filepath.Join(filepath.Dir(sshExe), "sftp-server"))

		assert.Equal(t, SftpServerForSSH(ctx, SSHExe{Exe: sshExe}), "",
			"unresolvable candidate -> caller falls back to sshocker auto-detect")
	})

	t.Run("cygpath failure returns empty", func(t *testing.T) {
		sshExe := cygwinToolchain(t)
		t.Setenv(fakeCygpathOutEnv, fakeCygpathFailure)

		assert.Equal(t, SftpServerForSSH(ctx, SSHExe{Exe: sshExe}), "")
	})
}

// TestCompanionForSSH: a companion tool is resolved beside the selected
// ssh.exe, and falls back to the bare name when no sibling exists.
func TestCompanionForSSH(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("companion resolution only runs on Windows")
	}

	t.Run("sibling is picked over PATH", func(t *testing.T) {
		dir := resolvedTempDir(t)
		sshExe := filepath.Join(dir, "ssh.exe")
		keygenExe := filepath.Join(dir, "ssh-keygen.exe")
		assert.NilError(t, os.WriteFile(sshExe, nil, 0o644))
		assert.NilError(t, os.WriteFile(keygenExe, nil, 0o644))

		assert.Equal(t, CompanionForSSH(SSHExe{Exe: sshExe}, "ssh-keygen"), keygenExe)
	})

	t.Run("no sibling falls back to the bare name", func(t *testing.T) {
		dir := resolvedTempDir(t)
		sshExe := filepath.Join(dir, "ssh.exe")
		assert.NilError(t, os.WriteFile(sshExe, nil, 0o644))

		assert.Equal(t, CompanionForSSH(SSHExe{Exe: sshExe}, "ssh-keygen"), "ssh-keygen")
	})

	t.Run("empty input falls back to the bare name", func(t *testing.T) {
		assert.Equal(t, CompanionForSSH(SSHExe{}, "ssh-keygen"), "ssh-keygen")
	})
}

// cygwinToolchain writes an ssh.exe into a fresh directory, plus a cygpath.exe
// that is a copy of this test binary, and returns the ssh.exe path.
func cygwinToolchain(t *testing.T) string {
	t.Helper()
	dir := resolvedTempDir(t)
	sshExe := filepath.Join(dir, "ssh.exe")
	assert.NilError(t, os.WriteFile(sshExe, nil, 0o644))

	self, err := os.Executable()
	assert.NilError(t, err)
	binary, err := os.ReadFile(self)
	assert.NilError(t, err)
	assert.NilError(t, os.WriteFile(filepath.Join(dir, "cygpath.exe"), binary, 0o755))
	return sshExe
}

// resolvedTempDir is t.TempDir() run through EvalSymlinks, matching what
// the production helpers compute. On GitHub Windows runners t.TempDir()
// is an 8.3 short path (C:\Users\RUNNER~1\...) that the helpers expand,
// so a raw compare would fail.
func resolvedTempDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	resolved, err := filepath.EvalSymlinks(dir)
	assert.NilError(t, err)
	return resolved
}
