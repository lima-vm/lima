//go:build !darwin
// +build !darwin

package hostagent

import (
	"context"

	"github.com/lima-vm/sshocker/pkg/ssh"
	"github.com/sirupsen/logrus"
)

func forwardTCP(ctx context.Context, l *logrus.Logger, sshConfig *ssh.SSHConfig, port int, local, remote string, cancel bool) error {
	return forwardSSH(ctx, sshConfig, port, local, remote, cancel)
}
