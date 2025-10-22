// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package hostagent

import (
	"context"

	"github.com/lima-vm/sshocker/pkg/ssh"
)

func forwardTCP(ctx context.Context, sshConfig *ssh.SSHConfig, addr string, port int, local, remote, verb string) error {
	return forwardSSH(ctx, sshConfig, addr, port, local, remote, verb, false)
}
