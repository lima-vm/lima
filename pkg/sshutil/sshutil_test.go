/*
Copyright The Lima Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

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
