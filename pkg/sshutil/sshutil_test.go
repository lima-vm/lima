// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package sshutil

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/coreos/go-semver/semver"
	"gotest.tools/v3/assert"
)

func TestDefaultPubKeys(t *testing.T) {
	keys, _ := DefaultPubKeys(t.Context(), true)
	t.Logf("found %d public keys", len(keys))
	for _, key := range keys {
		t.Logf("%s: %q", key.Filename, key.Content)
	}
}

// TestPickCompleteSSHOnWindows locks in the toolchain-completeness
// filter: an ssh.exe whose directory is missing scp.exe or
// ssh-keygen.exe (MinGit's shape) is skipped in favour of the next
// complete install on PATH.
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

// resolvedTempDir wraps t.TempDir() with EvalSymlinks so the test path
// matches what the production helpers (cygpathForSSH, SftpServerForSSH)
// compute after their own EvalSymlinks. On GitHub Windows runners
// t.TempDir() lives under C:\Users\RUNNER~1\... (8.3 short form),
// and the helpers expand it to C:\Users\runneradmin\... — a direct
// string compare would fail.
func resolvedTempDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	resolved, err := filepath.EvalSymlinks(dir)
	assert.NilError(t, err)
	return resolved
}

// TestCygpathForSSH locks in the toolchain-tracking contract: the
// cygpath that comes back from cygpathForSSH must be the sibling of
// the ssh.exe passed in, not whatever cygpath happens to be on PATH.
// PathForSSH passes that path straight to exec.Command, so a regression
// here would silently route conversions through the wrong toolchain.
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

// TestSftpServerForSSH locks in the toolchain-pairing contract: the
// sftp-server returned must come from the same install as sshExe.
// reverse-sshfs hands the result to sshocker's OpensshSftpServerBinary
// so the Windows path form (Cygwin vs native) matches the consumer.
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

func TestParseOpenSSHVersion(t *testing.T) {
	assert.Check(t, ParseOpenSSHVersion([]byte("OpenSSH_8.4p1 Ubuntu")).Equal(
		semver.Version{Major: 8, Minor: 4, Patch: 1, PreRelease: "", Metadata: ""}))

	assert.Check(t, ParseOpenSSHVersion([]byte("OpenSSH_7.6p1 Ubuntu")).LessThan(*semver.New("8.0.0")))

	// macOS 10.15
	assert.Check(t, ParseOpenSSHVersion([]byte("OpenSSH_8.1p1, LibreSSL 2.7.3")).Equal(*semver.New("8.1.1")))

	// OpenBSD 5.8
	assert.Check(t, ParseOpenSSHVersion([]byte("OpenSSH_7.0, LibreSSL")).Equal(*semver.New("7.0.0")))

	// NixOS 25.05
	assert.Check(t, ParseOpenSSHVersion([]byte(`command-line line 0: Unsupported option "gssapiauthentication"
OpenSSH_10.0p2, OpenSSL 3.4.1 11 Feb 2025`)).Equal(*semver.New("10.0.2")))

	// Native Windows OpenSSH (Win32-OpenSSH)
	assert.Check(t, ParseOpenSSHVersion([]byte("OpenSSH_for_Windows_9.5p2, LibreSSL 3.8.2")).Equal(*semver.New("9.5.2")))

	// Older Win32-OpenSSH releases separate "Windows" and the version with a space.
	assert.Check(t, ParseOpenSSHVersion([]byte("OpenSSH_for_Windows 9.5p2, LibreSSL 3.8.2")).Equal(*semver.New("9.5.2")))
}

func TestParseOpenSSHGSSAPISupported(t *testing.T) {
	assert.Check(t, parseOpenSSHGSSAPISupported("OpenSSH_8.4p1 Ubuntu"))
	assert.Check(t, !parseOpenSSHGSSAPISupported(`command-line line 0: Unsupported option "gssapiauthentication"
OpenSSH_10.0p2, OpenSSL 3.4.1 11 Feb 2025`))
}

func Test_detectValidPublicKey(t *testing.T) {
	assert.Check(t, detectValidPublicKey("ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAAACQDf2IooTVPDBw== 64bit"))
	assert.Check(t, detectValidPublicKey("ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAAACQDf2IooTVPDBw=="))
	assert.Check(t, detectValidPublicKey("ssh-dss AAAAB3NzaC1kc3MAAACBAP/yAytaYzqXq01uTd5+1RC=" /* truncate */))
	assert.Check(t, detectValidPublicKey("ecdsa-sha2-nistp256 AAAAE2VjZHNhLXNoYTItbmlzdHAyNTY=" /* truncate */))
	assert.Check(t, detectValidPublicKey("ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAICs1tSO/jx8oc4O=" /* truncate */))

	assert.Check(t, !detectValidPublicKey("wrong-algo AAAAB3NzaC1kc3MAAACBAP/yAytaYzqXq01uTd5+1RC="))
	assert.Check(t, !detectValidPublicKey("huge-length AAAD6A=="))
	assert.Check(t, !detectValidPublicKey("arbitrary content"))
	assert.Check(t, !detectValidPublicKey(""))
}

func Test_DisableControlMasterOptsFromSSHArgs(t *testing.T) {
	tests := []struct {
		name    string
		sshArgs []string
		want    []string
	}{
		{
			name: "no ControlMaster options",
			sshArgs: []string{
				"-o", "StrictHostKeyChecking=no", "-o", "UserKnownHostsFile=/dev/null",
			},
			want: []string{
				"-o", "ControlMaster=no", "-o", "ControlPath=none", "-o", "ControlPersist=no",
				"-o", "StrictHostKeyChecking=no", "-o", "UserKnownHostsFile=/dev/null",
			},
		},
		{
			name: "ControlMaster=yes",
			sshArgs: []string{
				"-o", "ControlMaster=yes", "-o", "UserKnownHostsFile=/dev/null",
			},
			want: []string{
				"-o", "ControlMaster=no", "-o", "ControlPath=none", "-o", "ControlPersist=no",
				"-o", "UserKnownHostsFile=/dev/null",
			},
		},
		{
			name: "ControlMaster=auto",
			sshArgs: []string{
				"-o", "ControlMaster=auto", "-o", "UserKnownHostsFile=/dev/null",
			},
			want: []string{
				"-o", "ControlMaster=no", "-o", "ControlPath=none", "-o", "ControlPersist=no",
				"-o", "UserKnownHostsFile=/dev/null",
			},
		},
		{
			name: "ControlMaster=auto with ControlPath",
			sshArgs: []string{
				"-o", "ControlMaster=auto", "-o", "ControlPath=/tmp/ssh-%r@%h:%p", "-o", "UserKnownHostsFile=/dev/null",
			},
			want: []string{
				"-o", "ControlMaster=no", "-o", "ControlPath=none", "-o", "ControlPersist=no",
				"-o", "UserKnownHostsFile=/dev/null",
			},
		},
		{
			name: "ControlPath only",
			sshArgs: []string{
				"-o", "ControlPath=/tmp/ssh-%r@%h:%p", "-o", "UserKnownHostsFile=/dev/null",
			},
			want: []string{
				"-o", "ControlMaster=no", "-o", "ControlPath=none", "-o", "ControlPersist=no",
				"-o", "UserKnownHostsFile=/dev/null",
			},
		},
		{
			name: "ControlMaster=no",
			sshArgs: []string{
				"-o", "ControlMaster=no", "-o", "UserKnownHostsFile=/dev/null",
			},
			want: []string{
				"-o", "ControlMaster=no", "-o", "ControlPath=none", "-o", "ControlPersist=no",
				"-o", "UserKnownHostsFile=/dev/null",
			},
		},
		{
			name: "ControlMaster=auto with other options",
			sshArgs: []string{
				"-o", "ControlMaster=auto", "-o", "StrictHostKeyChecking=no", "-o", "UserKnownHostsFile=/dev/null",
			},
			want: []string{
				"-o", "ControlMaster=no", "-o", "ControlPath=none", "-o", "ControlPersist=no",
				"-o", "StrictHostKeyChecking=no", "-o", "UserKnownHostsFile=/dev/null",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.DeepEqual(t, DisableControlMasterOptsFromSSHArgs(tt.sshArgs), tt.want)
		})
	}
}
