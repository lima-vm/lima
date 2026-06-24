// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"testing"

	"gotest.tools/v3/assert"
)

func TestValidateMountType(t *testing.T) {
	for _, ok := range []string{"", "virtiofs", "9p", "reverse-sshfs"} {
		assert.NilError(t, validateMountType(ok), "type %q should be valid", ok)
	}
	for _, bad := range []string{"nfs", "9P", "virtio-fs", "sshfs"} {
		assert.ErrorContains(t, validateMountType(bad), "invalid --type")
	}
}

func TestNewMountCommandSubcommands(t *testing.T) {
	cmd := newMountCommand()
	subs := map[string]bool{}
	for _, c := range cmd.Commands() {
		subs[c.Name()] = true
	}
	for _, want := range []string{"add", "remove", "list"} {
		assert.Assert(t, subs[want], "missing subcommand %q", want)
	}
}
