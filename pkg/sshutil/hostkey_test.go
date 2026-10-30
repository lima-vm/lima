// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package sshutil

import (
	"crypto/ed25519"
	"crypto/rand"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
	"gotest.tools/v3/assert"

	"github.com/lima-vm/lima/v2/pkg/limatype/filenames"
)

func generateHostKey(t *testing.T) ssh.Signer {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	assert.NilError(t, err)
	signer, err := ssh.NewSignerFromKey(priv)
	assert.NilError(t, err)
	return signer
}

// startSSHServer starts an SSH server with hostKey, which accepts any client without authentication.
func startSSHServer(t *testing.T, hostKey ssh.Signer) (address string, port int) {
	t.Helper()
	var lc net.ListenConfig
	l, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	assert.NilError(t, err)
	t.Cleanup(func() { _ = l.Close() })
	config := &ssh.ServerConfig{NoClientAuth: true}
	config.AddHostKey(hostKey)
	go func() {
		for {
			conn, err := l.Accept()
			if err != nil {
				return
			}
			go func() {
				defer conn.Close()
				_, _, _, _ = ssh.NewServerConn(conn, config)
			}()
		}
	}()
	tcpAddr := l.Addr().(*net.TCPAddr)
	return tcpAddr.IP.String(), tcpAddr.Port
}

func TestScanHostKey(t *testing.T) {
	hostKey := generateHostKey(t)
	address, port := startSSHServer(t, hostKey)
	scanned, err := ScanHostKey(t.Context(), address, port)
	assert.NilError(t, err)
	assert.DeepEqual(t, scanned.Marshal(), hostKey.PublicKey().Marshal())
}

func TestPinHostKey(t *testing.T) {
	instDir := t.TempDir()
	knownHostsPath := filepath.Join(instDir, filenames.SSHKnownHosts)
	hostKey := generateHostKey(t)
	address, port := startSSHServer(t, hostKey)

	// Not pinned yet
	pinned, err := ReadPinnedHostKey(instDir)
	assert.NilError(t, err)
	assert.Assert(t, pinned == nil)
	assert.NilError(t, WriteKnownHostsFile(instDir, "foo", address, port))
	b, err := os.ReadFile(knownHostsPath)
	assert.NilError(t, err)
	assert.Equal(t, len(b), 0)

	// The first boot pins the key.
	assert.NilError(t, PinHostKey(t.Context(), instDir, "foo", address, port))
	pinned, err = ReadPinnedHostKey(instDir)
	assert.NilError(t, err)
	assert.DeepEqual(t, pinned.Marshal(), hostKey.PublicKey().Marshal())

	// The later boots verify the key.
	assert.NilError(t, PinHostKey(t.Context(), instDir, "foo", address, port))
	otherAddress, otherPort := startSSHServer(t, generateHostKey(t))
	assert.ErrorIs(t, PinHostKey(t.Context(), instDir, "foo", otherAddress, otherPort), ErrHostKeyMismatch)
	pinned, err = ReadPinnedHostKey(instDir)
	assert.NilError(t, err)
	assert.DeepEqual(t, pinned.Marshal(), hostKey.PublicKey().Marshal())

	// The pinned key is looked up by the alias and the current address.
	assert.NilError(t, WriteKnownHostsFile(instDir, "foo", "127.0.0.1", 60022))
	callback, err := knownhosts.New(knownHostsPath)
	assert.NilError(t, err)
	remote := &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 60022}
	otherKey := generateHostKey(t).PublicKey()
	for _, host := range []string{"lima-foo:22", "127.0.0.1:60022"} {
		assert.NilError(t, callback(host, remote, hostKey.PublicKey()), host)
		assert.ErrorContains(t, callback(host, remote, otherKey), "key mismatch", host)
	}
	assert.ErrorContains(t, callback("127.0.0.1:60023", remote, hostKey.PublicKey()), "key is unknown")

	// The host names in the file are ignored, e.g., after cloning the instance.
	assert.NilError(t, WriteKnownHostsFile(instDir, "bar", "127.0.0.1", 60023))
	pinned, err = ReadPinnedHostKey(instDir)
	assert.NilError(t, err)
	assert.DeepEqual(t, pinned.Marshal(), hostKey.PublicKey().Marshal())
	b, err = os.ReadFile(knownHostsPath)
	assert.NilError(t, err)
	assert.Assert(t, strings.HasPrefix(string(b), "lima-bar,[127.0.0.1]:60023 "), string(b))
}

func TestRefreshKnownHostsFile(t *testing.T) {
	instDir := t.TempDir()
	knownHostsPath := filepath.Join(instDir, filenames.SSHKnownHosts)

	// The file is not created for an instance without a pinned key.
	assert.NilError(t, RefreshKnownHostsFile(instDir, "foo", "127.0.0.1", 60022))
	_, err := os.Stat(knownHostsPath)
	assert.ErrorIs(t, err, os.ErrNotExist)
	assert.NilError(t, WriteKnownHostsFile(instDir, "foo", "127.0.0.1", 60022))
	assert.NilError(t, RefreshKnownHostsFile(instDir, "foo", "127.0.0.1", 60022))
	b, err := os.ReadFile(knownHostsPath)
	assert.NilError(t, err)
	assert.Equal(t, len(b), 0)

	hostKey := generateHostKey(t)
	address, port := startSSHServer(t, hostKey)
	assert.NilError(t, PinHostKey(t.Context(), instDir, "foo", address, port))
	assert.NilError(t, RefreshKnownHostsFile(instDir, "foo", "127.0.0.1", 60023))
	callback, err := knownhosts.New(knownHostsPath)
	assert.NilError(t, err)
	remote := &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 60023}
	assert.NilError(t, callback("127.0.0.1:60023", remote, hostKey.PublicKey()))
	assert.ErrorContains(t, callback(net.JoinHostPort(address, strconv.Itoa(port)), remote, hostKey.PublicKey()), "key is unknown")
}

func TestRemoveKnownHostsFileUnlessPinned(t *testing.T) {
	instDir := t.TempDir()
	knownHostsPath := filepath.Join(instDir, filenames.SSHKnownHosts)

	assert.NilError(t, RemoveKnownHostsFileUnlessPinned(instDir))
	assert.NilError(t, WriteKnownHostsFile(instDir, "foo", "127.0.0.1", 60022))
	assert.NilError(t, RemoveKnownHostsFileUnlessPinned(instDir))
	_, err := os.Stat(knownHostsPath)
	assert.ErrorIs(t, err, os.ErrNotExist)

	hostKey := generateHostKey(t)
	address, port := startSSHServer(t, hostKey)
	assert.NilError(t, PinHostKey(t.Context(), instDir, "foo", address, port))
	assert.NilError(t, RemoveKnownHostsFileUnlessPinned(instDir))
	pinned, err := ReadPinnedHostKey(instDir)
	assert.NilError(t, err)
	assert.DeepEqual(t, pinned.Marshal(), hostKey.PublicKey().Marshal())
}

func TestHostKeyOpts(t *testing.T) {
	instDirA := filepath.Join(t.TempDir(), "a")
	instDirB := filepath.Join(t.TempDir(), "b")
	for _, instDir := range []string{instDirA, instDirB} {
		assert.NilError(t, os.MkdirAll(instDir, 0o700))
	}

	legacyOpts := []string{"StrictHostKeyChecking=no", "UserKnownHostsFile=/dev/null"}
	opts, err := HostKeyOpts(t.Context(), "ssh", instDirA)
	assert.NilError(t, err)
	assert.DeepEqual(t, opts, legacyOpts)

	// The host key is verified even before pinning the key.
	assert.NilError(t, WriteKnownHostsFile(instDirA, "a", "127.0.0.1", 60022))
	opts, err = HostKeyOpts(t.Context(), "ssh", instDirA)
	assert.NilError(t, err)
	assert.Assert(t, slices.Contains(opts, "StrictHostKeyChecking=yes"), "opts: %v", opts)
	assert.Assert(t, slices.Contains(opts, "HostKeyAlias=lima-a"), "opts: %v", opts)

	// Instance b has no known_hosts file yet, but instance a is still verified.
	opts, err = HostKeyOpts(t.Context(), "ssh", instDirA, instDirB)
	assert.NilError(t, err)
	assert.Assert(t, slices.Contains(opts, "StrictHostKeyChecking=accept-new"), "opts: %v", opts)
	idx := slices.IndexFunc(opts, func(o string) bool { return strings.HasPrefix(o, "UserKnownHostsFile=") })
	assert.Assert(t, idx >= 0, "opts: %v", opts)
	// ssh records the new keys in the first file.
	assert.Assert(t, strings.HasPrefix(opts[idx], "UserKnownHostsFile=/dev/null "), opts[idx])
	assert.Assert(t, strings.Contains(opts[idx], filepath.ToSlash(filepath.Join("a", filenames.SSHKnownHosts))), opts[idx])
	assert.Assert(t, !strings.Contains(opts[idx], filepath.ToSlash(filepath.Join("b", filenames.SSHKnownHosts))), opts[idx])

	assert.NilError(t, WriteKnownHostsFile(instDirB, "b", "127.0.0.1", 60023))
	opts, err = HostKeyOpts(t.Context(), "ssh", instDirA, instDirB)
	assert.NilError(t, err)
	assert.Assert(t, slices.Contains(opts, "StrictHostKeyChecking=yes"), "opts: %v", opts)
	assert.Assert(t, !slices.ContainsFunc(opts, func(o string) bool { return strings.HasPrefix(o, "HostKeyAlias=") }), "opts: %v", opts)
	idx = slices.IndexFunc(opts, func(o string) bool { return strings.HasPrefix(o, "UserKnownHostsFile=") })
	assert.Assert(t, idx >= 0, "opts: %v", opts)
	assert.Assert(t, !strings.Contains(opts[idx], "/dev/null"), opts[idx])
	assert.Assert(t, strings.Contains(opts[idx], filepath.ToSlash(filepath.Join("a", filenames.SSHKnownHosts))), opts[idx])
	assert.Assert(t, strings.Contains(opts[idx], filepath.ToSlash(filepath.Join("b", filenames.SSHKnownHosts))), opts[idx])
}
