// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package instance

import (
	"errors"
	"os/exec"
)

func execHostAgentForeground(_ string, _ *exec.Cmd) error {
	return errors.New("`limactl start --foreground` is not supported on Windows")
}
