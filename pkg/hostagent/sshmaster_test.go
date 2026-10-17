// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package hostagent

import (
	"testing"

	"gotest.tools/v3/assert"
)

func TestSSHMasterArgs(t *testing.T) {
	clientArgs := []string{
		"-F", "/dev/null",
		"-o", "IdentityFile=/key",
		"-o", "ControlMaster=no",
		"-o", `ControlPath="/lima/default/ssh.sock"`,
		"-o", "ControlPersist=yes",
		"-o", "User=me",
	}
	want := []string{
		"-o", "ControlMaster=yes",
		"-o", "ControlPersist=no",
		"-o", "ServerAliveInterval=15",
		"-o", "ServerAliveCountMax=4",
		"-F", "/dev/null",
		"-o", "IdentityFile=/key",
		"-o", `ControlPath="/lima/default/ssh.sock"`,
		"-o", "User=me",
		"-N", "-p", "60022", "127.0.0.1",
	}
	assert.DeepEqual(t, sshMasterArgs(clientArgs, "127.0.0.1", 60022), want)
}

func TestSSHMasterArgsWithoutPort(t *testing.T) {
	assert.DeepEqual(t, sshMasterArgs(nil, "127.0.0.1", 0)[8:], []string{"-N", "127.0.0.1"})
}
