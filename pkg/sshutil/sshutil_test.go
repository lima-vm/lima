// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package sshutil

import (
	"testing"

	"github.com/coreos/go-semver/semver"
	"gotest.tools/v3/assert"
)

func TestDefaultPubKeys(t *testing.T) {
	keys, _ := DefaultPubKeys(true)
	t.Logf("found %d public keys", len(keys))
	for _, key := range keys {
		t.Logf("%s: %q", key.Filename, key.Content)
	}
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
