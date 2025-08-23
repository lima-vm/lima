//go:build !windows

// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package instance

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"github.com/mattn/go-isatty"
	"github.com/sirupsen/logrus"

	"github.com/lima-vm/lima/v2/pkg/osutil"
)

func execHostAgentForeground(limactl string, haCmd *exec.Cmd) error {
	haStdoutW, ok := haCmd.Stdout.(*os.File)
	if !ok {
		return fmt.Errorf("expected haCmd.Stdout to be *os.File, got %T", haCmd.Stdout)
	}
	haStderrW, ok := haCmd.Stderr.(*os.File)
	if !ok {
		return fmt.Errorf("expected haCmd.Stderr to be *os.File, got %T", haCmd.Stderr)
	}
	logrus.Info("Running the host agent in the foreground")
	if isatty.IsTerminal(os.Stdin.Fd()) || isatty.IsCygwinTerminal(os.Stdin.Fd()) {
		// Write message to standard log files to avoid confusing users
		message := "This log file is not used because `limactl start` was launched in the terminal with the `--foreground` option."
		if _, err := haStdoutW.WriteString(message); err != nil {
			return err
		}
		if _, err := haStderrW.WriteString(message); err != nil {
			return err
		}
	} else {
		if err := osutil.Dup2(int(haStdoutW.Fd()), syscall.Stdout); err != nil {
			return err
		}
		if err := osutil.Dup2(int(haStderrW.Fd()), syscall.Stderr); err != nil {
			return err
		}
	}
	return syscall.Exec(limactl, haCmd.Args, haCmd.Environ())
}
