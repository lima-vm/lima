// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package sshutil

import (
	"errors"
	"net"
	"os"
	"path/filepath"
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

// TestDefaultPubKeys_WithoutExistingKey tests DefaultPubKeys without an existing public key.
func TestDefaultPubKeys_WithoutExistingKey(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("LIMA_HOME", tmpDir)
	keys, err := DefaultPubKeys(t.Context(), false)
	assert.NilError(t, err)
	assert.Equal(t, 1, len(keys))
	// LimaDir resolves symlinks (e.g. /var -> /private/var on macOS), so
	// resolve tmpDir the same way before comparing the returned filename.
	resolvedTmp, err := filepath.EvalSymlinks(tmpDir)
	assert.NilError(t, err)
	assert.Equal(t, keys[0].Filename, filepath.Join(resolvedTmp, filenames.ConfigDir, filenames.UserPublicKey))
	assert.Assert(t, detectValidPublicKey(keys[0].Content), "generated key is not a valid public key: %#q", keys[0].Content)
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

	// Older Win32-OpenSSH, with a space before the version
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
	// Length prefix equal to the decoded length: sigLength (8) is not larger than
	// len(decodedKey) (8) but the format field runs to offset 4+8, past the buffer.
	assert.Check(t, !detectValidPublicKey("trailing-length AAAACAAAAAA="))
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
