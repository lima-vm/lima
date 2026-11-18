// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"strings"
	"testing"

	"gotest.tools/v3/assert"
)

func TestGenerateSystemdUnitGuestToGuestInterfaces(t *testing.T) {
	without, err := generateSystemdUnit(2222, "", 0, nil, false)
	assert.NilError(t, err)
	assert.Assert(t, !strings.Contains(string(without), "--guest-to-guest-interfaces"))

	empty, err := generateSystemdUnit(2222, "", 0, []string{}, false)
	assert.NilError(t, err)
	assert.Equal(t, string(empty), string(without))

	with, err := generateSystemdUnit(2222, "", 0, []string{"lima0", "lima1"}, false)
	assert.NilError(t, err)
	assert.Assert(t, strings.Contains(string(with), "--vsock-port 2222 --guest-to-guest-interfaces lima0,lima1 --runtime-dir="))
}

func TestInstallSystemdEmptyGuestToGuestInterfaces(t *testing.T) {
	cmd := newInstallSystemdCommand()
	assert.NilError(t, cmd.Flags().Parse([]string{"--guest-to-guest-interfaces", ""}))
	names, err := cmd.Flags().GetStringSlice("guest-to-guest-interfaces")
	assert.NilError(t, err)
	assert.Equal(t, len(names), 0)
}
