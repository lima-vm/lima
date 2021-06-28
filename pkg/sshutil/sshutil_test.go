package sshutil

import "testing"

func TestDefaultPubKeys(t *testing.T) {
	keys, _ := DefaultPubKeys()
	t.Logf("found %d public keys", len(keys))
	for _, key := range keys {
		t.Logf("%s: %q", key.Filename, key.Content)
	}
}
