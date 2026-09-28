//go:build darwin

// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"gotest.tools/v3/assert"

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
	assert.Assert(t, !strings.Contains(content, "lima-privileged-block-device"))
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
	assert.NilError(t, os.WriteFile(file, []byte(networkSudoers+"alice ALL=(root:wheel) NOPASSWD:NOSETENV: /opt/lima/libexec/lima/privileged/lima-privileged-block-device /dev/rdisk2\n"), 0o600))

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

	assert.Equal(t, hint, "run `/opt/lima/bin/limactl sudoers --block-device=/dev/rdisk2 >etc_sudoers.d_lima && sudo install -o root -g wheel -m 0444 etc_sudoers.d_lima \"/etc/sudoers.d/lima\"`; regeneration emits only the current user's --block-device entries; include every entry you still need and preserve other users' entries manually; regenerating the file revokes omitted grants")
	assert.Assert(t, !strings.HasSuffix(hint, ")"))
}

func TestSudoersActionCheckWarnsAboutInvalidNetwork(t *testing.T) {
	const helperEnv = "LIMA_TEST_SUDOERS_CHECK_WARNING"
	if os.Getenv(helperEnv) == "1" {
		cmd := newSudoersCommand()
		cmd.SetArgs([]string{"--check", "--block-device=/dev/rdisk2", os.Getenv("LIMA_TEST_SUDOERS_FILE")})
		assert.Assert(t, cmd.Execute() != nil)
		return
	}

	limaHome := t.TempDir()
	configDir := filepath.Join(limaHome, "_config")
	assert.NilError(t, os.Mkdir(configDir, 0o755))
	config := "paths:\n  socketVMNet: /does/not/exist\n  varRun: /private/var/run/lima\ngroup: invalid group\nnetworks: {}\n"
	assert.NilError(t, os.WriteFile(filepath.Join(configDir, "networks.yaml"), []byte(config), 0o600))
	checkFile := filepath.Join(limaHome, "installed.sudoers")
	assert.NilError(t, os.WriteFile(checkFile, []byte("# existing file\n"), 0o600))

	cmd := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestSudoersActionCheckWarnsAboutInvalidNetwork$")
	cmd.Env = append(os.Environ(), helperEnv+"=1", "LIMA_HOME="+limaHome, "LIMA_TEST_SUDOERS_FILE="+checkFile)
	output, err := cmd.CombinedOutput()
	assert.NilError(t, err, string(output))
	assert.Assert(t, strings.Contains(string(output), "Network entries omitted from sudoers check"), string(output))
	assert.Assert(t, strings.Contains(string(output), "invalid group `invalid group`"), string(output))
	assert.Assert(t, strings.Contains(string(output), "--check did not generate or install a replacement file"), string(output))
}
