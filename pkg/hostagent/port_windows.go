// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package hostagent

import (
	"context"

	"github.com/lima-vm/sshocker/pkg/ssh"
)

func forwardTCP(ctx context.Context, sshConfig *ssh.SSHConfig, port int, local, remote, verb string) error {
	return forwardSSH(ctx, sshConfig, port, local, remote, verb, false)
}
