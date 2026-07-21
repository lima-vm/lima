//go:build darwin

// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
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
	}, nil, true)
	assert.NilError(t, err)
	assert.Assert(t, strings.Contains(content, "%everyone ALL=(root:wheel) NOPASSWD:NOSETENV: /bin/mkdir -m 775 -p /private/var/run/lima"))
	assert.Assert(t, !strings.Contains(content, blockdevice.SudoOpenBlockDeviceCommand))
}

func TestVerifySudoersFileAllowsAdditionalActiveFragments(t *testing.T) {
	cfg := networks.Config{
		Paths: networks.Paths{
			VarRun: "/private/var/run/lima",
		},
		Group:    "everyone",
		Networks: map[string]networks.Network{},
	}
	networkSudoers, err := cfg.Sudoers()
	assert.NilError(t, err)

	file := t.TempDir() + "/lima.sudoers"
	assert.NilError(t, os.WriteFile(file, []byte(networkSudoers+"alice ALL=(root:wheel) NOPASSWD:NOSETENV: /opt/lima/bin/limactl sudo-open-block-device /dev/rdisk2\n"), 0o600))

	assert.NilError(t, verifySudoersFile(t.Context(), cfg, file, nil, true))
}

func TestVerifySudoersFileRejectsCommentedNetworkFragment(t *testing.T) {
	cfg := networks.Config{
		Paths: networks.Paths{
			VarRun: "/private/var/run/lima",
		},
		Group:    "everyone",
		Networks: map[string]networks.Network{},
	}
	networkSudoers, err := cfg.Sudoers()
	assert.NilError(t, err)

	file := t.TempDir() + "/lima.sudoers"
	assert.NilError(t, os.WriteFile(file, []byte("# "+strings.ReplaceAll(networkSudoers, "\n", "\n# ")), 0o600))

	err = verifySudoersFile(t.Context(), cfg, file, nil, true)
	assert.ErrorContains(t, err, "out of sync")
}

func TestSudoersCheckHintHasBalancedParentheses(t *testing.T) {
	hint := sudoersCheckHint("/opt/lima/bin/limactl", "/etc/sudoers.d/lima", []string{"/dev/rdisk2"})

	assert.Equal(t, hint, "run `/opt/lima/bin/limactl sudoers --block-device=/dev/rdisk2 >etc_sudoers.d_lima && sudo install -o root etc_sudoers.d_lima \"/etc/sudoers.d/lima\"`")
	assert.Assert(t, !strings.HasSuffix(hint, ")"))
}
