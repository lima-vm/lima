// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package sudoers

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"

	"github.com/sirupsen/logrus"
)

// Args is the one place that defines Lima's non-interactive sudo invocation
// shape. Callers own the actual helper command names and sudoers policy; this
// package only keeps the invocation mechanics consistent.
func Args(user, group string, args ...string) []string {
	sudoArgs := []string{"--user", user, "--group", group, "--non-interactive"}
	return append(sudoArgs, args...)
}

func NewCommand(ctx context.Context, user, group string, stdin io.Reader, stdout, stderr io.Writer, dir string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, "sudo", Args(user, group, args...)...)
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.Dir = dir
	return cmd
}

func Run(ctx context.Context, user, group string, stdin io.Reader, stdout, stderr io.Writer, dir string, args ...string) error {
	cmd := NewCommand(ctx, user, group, stdin, stdout, stderr, dir, args...)
	logrus.Debugf("Running: %v", cmd.Args)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to run %v: %w", cmd.Args, err)
	}
	return nil
}

// NOPASSWD renders a sudoers entry for one or more commands.
func NOPASSWD(subject, runAsUser, runAsGroup string, commands ...string) string {
	if len(commands) == 1 {
		return fmt.Sprintf("%s ALL=(%s:%s) NOPASSWD:NOSETENV: %s\n", subject, runAsUser, runAsGroup, commands[0])
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "%s ALL=(%s:%s) NOPASSWD:NOSETENV: \\\n", subject, runAsUser, runAsGroup)
	for i, command := range commands {
		fmt.Fprintf(&sb, "    %s", command)
		if i < len(commands)-1 {
			sb.WriteString(", \\\n")
		} else {
			sb.WriteByte('\n')
		}
	}
	return sb.String()
}

// AssembleSudoersFragments joins already-rendered sudoers fragments into a
// single file. The individual fragments stay owned by their domain packages;
// this helper only handles the generic newline normalization needed to
// concatenate them.
func AssembleSudoersFragments(fragments ...string) string {
	var sb strings.Builder
	for _, fragment := range fragments {
		if fragment == "" {
			continue
		}
		if sb.Len() > 0 && !strings.HasSuffix(sb.String(), "\n") {
			sb.WriteByte('\n')
		}
		sb.WriteString(fragment)
	}
	return sb.String()
}
