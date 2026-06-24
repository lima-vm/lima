//go:build darwin

// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"strings"
	"testing"

	"gotest.tools/v3/assert"

	"github.com/lima-vm/lima/v2/pkg/blockdevice"
	"github.com/lima-vm/lima/v2/pkg/networks"
)

func TestRenderSudoersOmitsBlockDeviceByDefault(t *testing.T) {
	content, err := renderSudoers(networks.Config{
		Paths: networks.Paths{
			VarRun: "/private/var/run/lima",
		},
		Group:    "everyone",
		Networks: map[string]networks.Network{},
	}, nil)
	assert.NilError(t, err)
	assert.Assert(t, strings.Contains(content, "%everyone ALL=(root:wheel) NOPASSWD:NOSETENV: /bin/mkdir -m 775 -p /private/var/run/lima"))
	assert.Assert(t, !strings.Contains(content, blockdevice.SudoOpenBlockDeviceCommand))
}
