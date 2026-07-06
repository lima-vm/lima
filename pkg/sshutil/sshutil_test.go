// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package sshutil

import (
	"errors"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/coreos/go-semver/semver"
	"gotest.tools/v3/assert"

	"github.com/lima-vm/lima/v2/pkg/limatype/filenames"
)

func TestDefaultPubKeys(t *testing.T) {
	keys, _ := DefaultPubKeys(t.Context(), true)
	t.Logf("found %d public keys", len(keys))
	for _, key := range keys {
		t.Logf("%s: %#q", key.Filename, key.Content)
	}
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

// TestCygpathForSSH: cygpathForSSH must return the cygpath beside the
// given ssh.exe, not one off PATH — PathForSSH runs it directly, so a
// regression would route conversions through the wrong toolchain.
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

// TestSftpServerForSSH: the sftp-server returned must come from the same
// install as sshExe, so its Windows path form matches what reverse-sshfs
// feeds to sshocker.
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

func TestControlMasterStale(t *testing.T) {
	// A leftover socket file with no listening master must be detected as
	// not-running and removed, so --reconnect can recover the wedged session.
	t.Run("stale socket is removed", func(t *testing.T) {
		dir := t.TempDir()
		sock := filepath.Join(dir, filenames.SSHSock)
		assert.NilError(t, os.WriteFile(sock, []byte{}, 0o600))

		assert.Check(t, !IsControlMasterRunning(t.Context(), dir))

		removed, err := RemoveStaleControlMaster(t.Context(), dir)
		assert.NilError(t, err)
		assert.Check(t, removed)

		_, statErr := os.Stat(sock)
		assert.Check(t, errors.Is(statErr, os.ErrNotExist))
	})

	// No socket at all is a no-op, not an error.
	t.Run("absent socket is a no-op", func(t *testing.T) {
		dir := t.TempDir()

		assert.Check(t, !IsControlMasterRunning(t.Context(), dir))

		removed, err := RemoveStaleControlMaster(t.Context(), dir)
		assert.NilError(t, err)
		assert.Check(t, !removed)
	})

	// A live master (a real listener on the socket) must be detected as running
	// and left untouched: only stale sockets are removed.
	t.Run("live master socket is preserved", func(t *testing.T) {
		dir := t.TempDir()
		sock := filepath.Join(dir, filenames.SSHSock)
		var lc net.ListenConfig
		ln, err := lc.Listen(t.Context(), "unix", sock)
		if err != nil {
			// Some platforms / temp-dir path lengths cannot host a unix socket
			// listener (e.g. macOS sun_path limit, Windows). The deterministic
			// stale and absent cases above still provide coverage there.
			t.Skipf("cannot create unix listener: %v", err)
		}
		defer ln.Close()

		assert.Check(t, IsControlMasterRunning(t.Context(), dir))

		removed, err := RemoveStaleControlMaster(t.Context(), dir)
		assert.NilError(t, err)
		assert.Check(t, !removed)

		_, statErr := os.Stat(sock)
		assert.NilError(t, statErr)
	})
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
