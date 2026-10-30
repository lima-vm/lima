// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package sshutil

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"

	"github.com/lima-vm/lima/v2/pkg/fsutil"
	"github.com/lima-vm/lima/v2/pkg/instance/hostname"
	"github.com/lima-vm/lima/v2/pkg/limatype/filenames"
)

// ErrHostKeyMismatch is returned by PinHostKey when the guest presents a host key that differs from the pinned key.
var ErrHostKeyMismatch = errors.New("host key mismatch")

// ReadPinnedHostKey returns the SSH host key pinned in the known_hosts file of the instance.
// It returns nil when the file does not exist or pins no key yet.
// The host names in the file are ignored, so that the file remains valid after cloning or renaming the instance.
func ReadPinnedHostKey(instDir string) (ssh.PublicKey, error) {
	b, err := os.ReadFile(filepath.Join(instDir, filenames.SSHKnownHosts))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	_, _, hostKey, _, _, err := ssh.ParseKnownHosts(b)
	if err != nil {
		if errors.Is(err, io.EOF) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to parse %#q: %w", filenames.SSHKnownHosts, err)
	}
	return hostKey, nil
}

// WriteKnownHostsFile rewrites the known_hosts file of the instance, so that the pinned SSH host key
// is looked up by both the "lima-<INSTANCE>" alias and sshAddress:sshPort.
// When no key is pinned yet, an empty file is written, so that HostKeyOpts does not disable
// the host key verification until PinHostKey pins the key.
func WriteKnownHostsFile(instDir, instName, sshAddress string, sshPort int) error {
	hostKey, err := ReadPinnedHostKey(instDir)
	if err != nil {
		return err
	}
	return writeKnownHostsFile(instDir, instName, hostKey, sshAddress, sshPort)
}

func writeKnownHostsFile(instDir, instName string, hostKey ssh.PublicKey, sshAddress string, sshPort int) error {
	var b []byte
	if hostKey != nil {
		addresses := []string{hostname.FromInstName(instName)}
		if sshAddress != "" {
			addresses = append(addresses, net.JoinHostPort(sshAddress, strconv.Itoa(sshPort)))
		}
		b = []byte(knownhosts.Line(addresses, hostKey) + "\n")
	}
	// Written atomically, as an empty file would let PinHostKey trust any key on the next boot.
	return fsutil.WriteFileAtomic(filepath.Join(instDir, filenames.SSHKnownHosts), b)
}

// RefreshKnownHostsFile is similar to WriteKnownHostsFile, but does nothing when no key is pinned.
// Used for updating the address and the port in the known_hosts file of an instance that may be running
// with an older version of the host agent, which does not update the file.
func RefreshKnownHostsFile(instDir, instName, sshAddress string, sshPort int) error {
	hostKey, err := ReadPinnedHostKey(instDir)
	if err != nil || hostKey == nil {
		return err
	}
	return writeKnownHostsFile(instDir, instName, hostKey, sshAddress, sshPort)
}

// RemoveKnownHostsFileUnlessPinned removes the known_hosts file of the instance when it pins no key,
// so that an older version of Lima that starts the instance next does not fail to verify the host key.
func RemoveKnownHostsFileUnlessPinned(instDir string) error {
	hostKey, err := ReadPinnedHostKey(instDir)
	if err != nil || hostKey != nil {
		return err
	}
	return os.RemoveAll(filepath.Join(instDir, filenames.SSHKnownHosts))
}

// PinHostKey verifies the SSH host key of the guest at sshAddress:sshPort against the known_hosts file of the instance.
// When the file pins no key yet, i.e., on the first boot after creating the instance,
// the key presented by the guest is trusted and pinned.
func PinHostKey(ctx context.Context, instDir, instName, sshAddress string, sshPort int) error {
	hostKey, err := ScanHostKey(ctx, sshAddress, sshPort)
	if err != nil {
		return fmt.Errorf("failed to get the SSH host key of the guest: %w", err)
	}
	pinned, err := ReadPinnedHostKey(instDir)
	if err != nil {
		return err
	}
	if pinned == nil {
		logrus.Infof("Pinning the SSH host key of the guest (%s) in %#q", ssh.FingerprintSHA256(hostKey), filenames.SSHKnownHosts)
		return writeKnownHostsFile(instDir, instName, hostKey, sshAddress, sshPort)
	}
	if !bytes.Equal(hostKey.Marshal(), pinned.Marshal()) {
		return fmt.Errorf("%w: the SSH host key of the guest (%s) does not match the key pinned in %#q (%s); "+
			"if the host key of the guest was changed intentionally, remove the file and restart the instance",
			ErrHostKeyMismatch, ssh.FingerprintSHA256(hostKey), filepath.Join(instDir, filenames.SSHKnownHosts), ssh.FingerprintSHA256(pinned))
	}
	return nil
}

// ScanHostKey returns the ssh-ed25519 host key of the SSH server at address:port, like ssh-keyscan.
// The server has proven the possession of the private key when the key is returned.
func ScanHostKey(ctx context.Context, address string, port int) (ssh.PublicKey, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	addr := net.JoinHostPort(address, strconv.Itoa(port))
	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	if deadline, ok := ctx.Deadline(); ok {
		if err := conn.SetDeadline(deadline); err != nil {
			return nil, err
		}
	}
	errScanned := errors.New("scanned the host key")
	var hostKey ssh.PublicKey
	config := &ssh.ClientConfig{
		HostKeyAlgorithms: []string{ssh.KeyAlgoED25519},
		// Called after verifying the signature made with the host key.
		HostKeyCallback: func(_ string, _ net.Addr, key ssh.PublicKey) error {
			hostKey = key
			return errScanned
		},
	}
	if _, _, _, err := ssh.NewClientConn(conn, addr, config); hostKey == nil {
		return nil, err
	}
	return hostKey, nil
}

// HostKeyOpts returns ssh option key-value pairs for verifying the SSH host keys of the instances.
// With a single instance, the key is looked up by the "lima-<INSTANCE>" alias, which does not
// depend on the SSH address and port. With multiple instances, the key is looked up by the address and port.
//
// An instance without the known_hosts file, i.e., an instance that has never been started by this version of Lima,
// is not verified. When none of the instances has the file, the result disables the host key verification.
// Otherwise, the keys of the instances without the file are accepted with StrictHostKeyChecking=accept-new,
// which still refuses a key that does not match the pinned key, and are recorded in /dev/null,
// as ssh records new keys in the first UserKnownHostsFile.
//
// Path options take toolPath's form, as described on CommonOpts.
func HostKeyOpts(ctx context.Context, toolPath string, instDirs ...string) ([]string, error) {
	var knownHostsFiles []string
	for _, instDir := range instDirs {
		knownHostsPath := filepath.Join(instDir, filenames.SSHKnownHosts)
		if _, err := os.Stat(knownHostsPath); err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				logrus.Debugf("%#q does not exist, disabling SSH host key verification for the instance", knownHostsPath)
				continue
			}
			return nil, err
		}
		knownHostsPath, err := PathForTool(ctx, toolPath, knownHostsPath)
		if err != nil {
			return nil, err
		}
		if runtime.GOOS == "windows" {
			knownHostsFiles = append(knownHostsFiles, fmt.Sprintf(`'%s'`, knownHostsPath))
		} else {
			knownHostsFiles = append(knownHostsFiles, fmt.Sprintf(`"%s"`, knownHostsPath))
		}
	}
	if len(knownHostsFiles) == 0 {
		return []string{
			"StrictHostKeyChecking=no",
			"UserKnownHostsFile=/dev/null",
		}, nil
	}
	strictHostKeyChecking := "yes"
	if len(knownHostsFiles) < len(instDirs) {
		strictHostKeyChecking = "accept-new"
		knownHostsFiles = append([]string{"/dev/null"}, knownHostsFiles...)
	}
	opts := []string{
		"StrictHostKeyChecking=" + strictHostKeyChecking,
		"UserKnownHostsFile=" + strings.Join(knownHostsFiles, " "),
		"HostKeyAlgorithms=ssh-ed25519",
	}
	if len(instDirs) == 1 {
		opts = append(opts, "HostKeyAlias="+hostname.FromInstName(filepath.Base(instDirs[0])))
	}
	return opts, nil
}
