// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"testing"

	"gotest.tools/v3/assert"
)

func TestVerifyArgs(t *testing.T) {
	assert.NilError(t, verifyArgs([]string{"tap", "--bridge=lima-host", "--network=host", "--", "default"}))

	// Joined with spaces, these satisfy the sudoers entry
	// "tap --bridge=lima-host --network=host -- *" too, but the "--" is part of
	// a flag value, and the smuggled --bridge overrides the pinned one.
	for _, args := range [][]string{
		{"tap", "--bridge=lima-host", "--network=host --", "--bridge=docker0", "--network=host", "default"},
		{"tap", "--bridge=lima-host --network=host --", "--bridge=docker0", "--network=host", "default"},
		{"tap", "--bridge=lima-host", "--network=host", "--", "default\t--bridge=docker0"},
	} {
		assert.ErrorContains(t, verifyArgs(args), "contains whitespace")
	}
}
