// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package networks

import (
	"testing"

	"gotest.tools/v3/assert"
)

func TestValidateRejectsInjectableNetworkDefinitions(t *testing.T) {
	// A newline in a network name adds a directive to the generated sudoers file.
	err := (&Config{Networks: map[string]Network{
		"evil\n%staff ALL=(root) NOPASSWD: ALL": {Mode: ModeShared},
	}}).Validate()
	assert.ErrorContains(t, err, "invalid network name")

	// A space in the interface injects an extra argument into the sudo command.
	err = (&Config{Networks: map[string]Network{
		"bridged": {Mode: ModeBridged, Interface: "en0 --extra-root-flag"},
	}}).Validate()
	assert.ErrorContains(t, err, "invalid interface")

	// Whitespace in the mode injects an extra argument into the sudo command.
	err = (&Config{Networks: map[string]Network{
		"shared": {Mode: "shared --extra"},
	}}).Validate()
	assert.ErrorContains(t, err, "invalid mode")

	// Whitespace in the group breaks the sudoers header line.
	err = (&Config{Group: "admin bad", Networks: map[string]Network{}}).Validate()
	assert.ErrorContains(t, err, "invalid group")

	// A path-traversal network name would redirect the pidfile/socket path.
	err = (&Config{Networks: map[string]Network{
		"../../etc/foo": {Mode: ModeShared},
	}}).Validate()
	assert.ErrorContains(t, err, "invalid network name")
}
