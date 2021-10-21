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
