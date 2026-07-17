// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package sshutil

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"gotest.tools/v3/assert"
)

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
		assert.Equal(t, IsSSHCygwin(SSHExe{Exe: sshExe}), true)
	})

	t.Run("native", func(t *testing.T) {
		dir := resolvedTempDir(t)
		sshExe := filepath.Join(dir, "ssh.exe")
		assert.NilError(t, os.WriteFile(sshExe, nil, 0o644))

		got, ok := cygpathForSSH(SSHExe{Exe: sshExe})
		assert.Equal(t, ok, false, "ssh.exe with no sibling cygpath.exe should be native")
		assert.Equal(t, got, "")
		assert.Equal(t, IsSSHCygwin(SSHExe{Exe: sshExe}), false)
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

		assert.Equal(t, companionForSSH(SSHExe{Exe: sshExe}, "ssh-keygen"), keygenExe)
	})

	t.Run("no sibling falls back to the bare name", func(t *testing.T) {
		dir := resolvedTempDir(t)
		sshExe := filepath.Join(dir, "ssh.exe")
		assert.NilError(t, os.WriteFile(sshExe, nil, 0o644))

		assert.Equal(t, companionForSSH(SSHExe{Exe: sshExe}, "ssh-keygen"), "ssh-keygen")
	})

	t.Run("empty input falls back to the bare name", func(t *testing.T) {
		assert.Equal(t, companionForSSH(SSHExe{}, "ssh-keygen"), "ssh-keygen")
	})
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
