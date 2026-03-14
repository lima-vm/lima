// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package osutil

import (
	"os"
	"os/exec"
)

// HandleExitError calls os.Exit immediately without printing an error, only if the error is an *exec.ExitError (non-nil).
//
// The function does not call os.Exit if the error is of any other type, even if it wraps an *exec.ExitError,
// so that the caller can print the error message.
func HandleExitError(err error) {
	if err == nil {
		return
	}

	// Do not use errors.As, because we want to match only *exec.ExitError, not wrapped ones.
	// https://github.com/lima-vm/lima/pull/4168
	if exitErr, ok := err.(*exec.ExitError); ok {
		os.Exit(exitErr.ExitCode()) //nolint:revive // it's intentional to call os.Exit in this function
		return
	}
}
