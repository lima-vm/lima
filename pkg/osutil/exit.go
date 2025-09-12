// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package osutil

import (
	"errors"
	"os"
	"os/exec"
)

func HandleExitError(err error) {
	if err == nil {
		return
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		os.Exit(exitErr.ExitCode()) //nolint:revive // it's intentional to call os.Exit in this function
		return
	}
}
