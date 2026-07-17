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
