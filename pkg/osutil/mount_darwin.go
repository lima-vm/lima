// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package osutil

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/sirupsen/logrus"
)

// Mount mounts the device.
// Root privileges is not necessary.
func Mount(ctx context.Context, fs, dev, mnt string, options []string) error {
	args := []string{"-t", fs}
	if len(options) > 0 {
		args = append(args, "-o", strings.Join(options, ","))
	}
	args = append(args, dev, mnt)
	cmd := exec.CommandContext(ctx, "mount", args...)
	logrus.Debugf("Executing command: %v", cmd.Args)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to mount %q on %q: %w (output=%q)", dev, mnt, err, output)
	}
	return nil
}

func Umount(ctx context.Context, mnt string) error {
	cmd := exec.CommandContext(ctx, "umount", mnt)
	logrus.Debugf("Executing command: %v", cmd.Args)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to unmount %q: %w (output=%q)", mnt, err, output)
	}
	return nil
}
