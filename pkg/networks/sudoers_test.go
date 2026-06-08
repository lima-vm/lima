// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package networks

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gotest.tools/v3/assert"
)

func TestVerifySudoAccessAllowsAdditionalFragments(t *testing.T) {
	cfg := Config{
		Group:    "everyone",
		Networks: map[string]Network{},
		Paths: Paths{
			VarRun: "/private/var/run/lima",
		},
	}

	networkSudoers, err := cfg.Sudoers()
	assert.NilError(t, err)

	composed := networkSudoers
	if composed != "" && !strings.HasSuffix(composed, "\n") {
		composed += "\n"
	}
	composed += "%everyone ALL=(root:wheel) NOPASSWD:NOSETENV: /usr/local/bin/limactl sudo-open-block-device\n"

	sudoersFile := filepath.Join(t.TempDir(), "lima.sudoers")
	assert.NilError(t, os.WriteFile(sudoersFile, []byte(composed), 0o600))

	assert.NilError(t, cfg.VerifySudoAccess(t.Context(), sudoersFile))
}

func TestVerifySudoAccessRejectsOutOfSyncNetworkFragment(t *testing.T) {
	cfg := Config{
		Group:    "everyone",
		Networks: map[string]Network{},
		Paths: Paths{
			VarRun: "/private/var/run/lima",
		},
	}

	networkSudoers, err := cfg.Sudoers()
	assert.NilError(t, err)
	assert.Assert(t, strings.Contains(networkSudoers, "NOPASSWD:NOSETENV"))

	modified := strings.Replace(networkSudoers, "NOPASSWD:NOSETENV", "NOPASSWD", 1)
	sudoersFile := filepath.Join(t.TempDir(), "lima.sudoers")
	assert.NilError(t, os.WriteFile(sudoersFile, []byte(modified), 0o600))

	err = cfg.VerifySudoAccess(t.Context(), sudoersFile)
	assert.ErrorContains(t, err, "out of sync")
}
