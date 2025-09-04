// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package executil

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/sirupsen/logrus"

	"github.com/lima-vm/lima/v2/pkg/ioutilx"
	"github.com/lima-vm/lima/v2/pkg/usrlocalsharelima"
)

type options struct {
	ctx context.Context
}

type Opt func(*options) error

// WithContext runs the command with CommandContext.
func WithContext(ctx context.Context) Opt {
	return func(o *options) error {
		o.ctx = ctx
		return nil
	}
}

func RunUTF16leCommand(args []string, opts ...Opt) (string, error) {
	var o options
	for _, f := range opts {
		if err := f(&o); err != nil {
			return "", err
		}
	}

	var cmd *exec.Cmd
	ctx := o.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	cmd = exec.CommandContext(ctx, args[0], args[1:]...)

	outString := ""
	out, err := cmd.CombinedOutput()
	if out != nil {
		s, err := ioutilx.FromUTF16leToString(bytes.NewReader(out))
		if err != nil {
			return "", fmt.Errorf("failed to convert output from UTF16 when running command %v, err: %w", args, err)
		}
		outString = s
	}
	return outString, err
}

// WithExecutablePath prepends the directory containing the current executable to the PATH
// and appends "/usr/local/libexec/lima" to the end before calling fn().
//
// This can be used to prefer plugins from the same directory over ones on the PATH and also works if the
// directory containing the executable itself is not on the path (e.g. "./_output/bin/limactl").
//
// It means if plugins call limactl they will invoke back the executable that called them.
func WithExecutablePath(fn func() error) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	currentPath := os.Getenv("PATH")
	defer os.Setenv("PATH", currentPath)
	newPath := filepath.Dir(exe) + string(filepath.ListSeparator) + currentPath

	prefixDir, err := usrlocalsharelima.Prefix()
	if err == nil {
		newPath += string(filepath.ListSeparator) + filepath.Join(prefixDir, "libexec", "lima")
	} else {
		// This happens in Go unit tests: https://github.com/lima-vm/lima/issues/3208
		logrus.Warnf("Couldn't locate libexec path: %v", err)
	}

	if err := os.Setenv("PATH", newPath); err != nil {
		return fmt.Errorf("failed to set PATH environment: %w", err)
	}

	return fn()
}
