//go:build !darwin

// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package hostagent

import (
	"context"
	"os/exec"
)

func sshCommand(ctx context.Context, sshBinary string, args ...string) *exec.Cmd {
	return exec.CommandContext(ctx, sshBinary, args...)
}
