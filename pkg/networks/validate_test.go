// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package networks

import (
	"errors"
	"os"
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

func TestValidateRejectsInjectableVarRun(t *testing.T) {
	// findBaseDirectory() trims the components that don't exist yet, so the
	// injected sudoers directive never reached the path check.
	err := (&Config{Paths: Paths{
		VarRun: "/private/var/run/lima\n%staff ALL=(root:wheel) NOPASSWD:NOSETENV: ALL",
	}}).Validate()
	assert.ErrorContains(t, err, "invalid component")

	// A space in the same position injects an extra argument into the sudo command.
	err = (&Config{Paths: Paths{
		VarRun: "/private/var/run/lima --extra-root-flag",
	}}).Validate()
	assert.ErrorContains(t, err, "invalid component")

	// A comma ends the command list entry, so "ALL" becomes a command of its own.
	err = (&Config{Paths: Paths{
		VarRun: "/private/var/run/lima,ALL",
	}}).Validate()
	assert.ErrorContains(t, err, "invalid component")
}

func TestValidateVarRunString(t *testing.T) {
	for _, path := range []string{
		"",
		"/",
		"/private/var/run/lima",
		"/private/var/run/lima/",
		"/private/etc/sudoers.d/lima",
		"/opt/socket_vmnet/bin/socket_vmnet",
		"/opt/homebrew/opt/socket_vmnet/bin/socket_vmnet",
	} {
		assert.NilError(t, validateVarRunString(path), path)
	}
	for _, path := range []string{
		"private/var/run/lima",
		"/private/var/run/lima ALL",
		"/private/var/run/lima\tALL",
		"/private/var/run/lima\nALL",
		"/private/var/run/lima,ALL",
		"/private/var/run/lima:ALL",
		"/private/var/run/lima\\ ALL",
		"/private/var/run/../../etc",
	} {
		assert.Assert(t, validateVarRunString(path) != nil, path)
	}
}

// The stricter identifier rule is applied to varRun only. socketVMNet and sudoers
// keep the whitespace check from master, so layouts with dotted components (which
// identifiers.Validate rejects) must still pass validatePath's string checks.
func TestValidatePathAllowsDottedComponents(t *testing.T) {
	for _, path := range []string{
		"/lima-nonexistent-test/foo/.local/bin/socket_vmnet",
		"/lima-nonexistent-test/etc/sudoers.d/lima",
	} {
		// os.Lstat runs after the string checks and fails because the path does
		// not exist; reaching that error means the dotted component passed the
		// stricter identifier rule that would apply to varRun.
		err := validatePath(path, false)
		assert.Assert(t, errors.Is(err, os.ErrNotExist), "%s: %v", path, err)
	}

	err := validatePath("/private/var/run/lima ALL", false)
	assert.ErrorContains(t, err, "contains whitespace")
}
