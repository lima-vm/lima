// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package hostagent

import (
	"context"
	"os/exec"
)

// sshCommand returns a command that runs ssh with its soft fd limit set to at most 1024.
// macOS ssh polls with select(), which rejects fds >= FD_SETSIZE (1024), so an ssh
// that is allowed more fds quits once it holds fd 1024.
func sshCommand(ctx context.Context, sshBinary string, args ...string) *exec.Cmd {
	// Resolve ssh with os/exec. When PATH has a relative entry such as ".", the
	// shell would run an ssh from the working directory, but os/exec refuses.
	sshPath, err := exec.LookPath(sshBinary)
	cmd := exec.CommandContext(ctx, "/bin/sh", append([]string{"-c", "ulimit -S -n 1024 2>/dev/null; exec \"$0\" \"$@\"", sshPath}, args...)...)
	// When the lookup fails, Start returns err without starting sh or ssh.
	if err != nil {
		cmd.Err = err
	}
	return cmd
}
