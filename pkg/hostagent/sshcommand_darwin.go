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
	return exec.CommandContext(ctx, "/bin/sh", append([]string{"-c", "ulimit -S -n 1024 2>/dev/null; exec \"$0\" \"$@\"", sshBinary}, args...)...)
}
