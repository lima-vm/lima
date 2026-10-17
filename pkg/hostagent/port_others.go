//go:build !darwin && !windows

// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package hostagent

import (
	"context"

	"github.com/lima-vm/sshocker/pkg/ssh"
)

func forwardTCP(ctx context.Context, sshConfig *ssh.SSHConfig, sshAddress string, sshPort int, local, remote, verb string) error {
	return forwardSSH(ctx, sshConfig, sshAddress, sshPort, local, remote, verb, false)
}

// replayForwardTCP restores a TCP forward on a fresh master. It cancels first,
// since the fresh master may already hold the forward.
func replayForwardTCP(ctx context.Context, sshConfig *ssh.SSHConfig, sshAddress string, sshPort int, local, remote string) error {
	_ = forwardTCP(ctx, sshConfig, sshAddress, sshPort, local, remote, verbCancel)
	return forwardTCP(ctx, sshConfig, sshAddress, sshPort, local, remote, verbForward)
}
