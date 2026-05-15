// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/lima-vm/lima/v2/pkg/autostart/launchd"
	"github.com/lima-vm/lima/v2/pkg/store"
	"github.com/lima-vm/lima/v2/pkg/textutil"
)

func daemonInstallAction(cmd *cobra.Command, args []string) error {
	userName, err := cmd.Flags().GetString("user")
	if err != nil {
		return err
	}
	if userName == "" {
		userName = os.Getenv("USER")
	}
	if userName == "" {
		return errors.New("could not determine user; pass --user")
	}

	ctx := cmd.Context()
	inst, err := store.Inspect(ctx, args[0])
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("instance %q not found", args[0])
		}
		return err
	}

	selfExe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("could not determine limactl path: %w", err)
	}

	content, err := textutil.ExecuteTemplate(launchd.DaemonTemplate, map[string]string{
		"Binary":   selfExe,
		"Instance": inst.Name,
		"WorkDir":  inst.Dir,
		"UserName": userName,
	})
	if err != nil {
		return fmt.Errorf("failed to render daemon plist: %w", err)
	}

	tmp, err := os.CreateTemp("", "io.lima-vm.daemon.*.plist")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(content); err != nil {
		return fmt.Errorf("failed to write temp plist: %w", err)
	}
	tmp.Close()

	destPath := launchd.GetDaemonPlistPath(inst.Name)
	svcTarget := "system/" + launchd.DaemonServiceNameFrom(inst.Name)

	// Bootout first in case a previous install is still loaded (ignore error).
	_ = runSudo(ctx, "launchctl", "bootout", svcTarget)

	if err := runSudo(ctx, "install", "-m", "644", tmp.Name(), destPath); err != nil {
		return fmt.Errorf("failed to install plist to %s: %w", destPath, err)
	}
	if err := runSudo(ctx, "launchctl", "enable", svcTarget); err != nil {
		return fmt.Errorf("failed to enable LaunchDaemon: %w", err)
	}
	if err := runSudo(ctx, "launchctl", "bootstrap", "system", destPath); err != nil {
		return fmt.Errorf("failed to bootstrap LaunchDaemon: %w", err)
	}

	logrus.Infof("LaunchDaemon installed for instance %q (runs as %q at boot)", inst.Name, userName)
	logrus.Infof("Plist: %s", destPath)
	return nil
}

func daemonUninstallAction(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	inst, err := store.Inspect(ctx, args[0])
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("instance %q not found", args[0])
		}
		return err
	}

	svcTarget := "system/" + launchd.DaemonServiceNameFrom(inst.Name)
	destPath := launchd.GetDaemonPlistPath(inst.Name)

	_ = runSudo(ctx, "launchctl", "bootout", svcTarget)
	_ = runSudo(ctx, "launchctl", "disable", svcTarget)
	if err := runSudo(ctx, "rm", "-f", destPath); err != nil {
		return fmt.Errorf("failed to remove %s: %w", destPath, err)
	}
	logrus.Infof("LaunchDaemon uninstalled for instance %q", inst.Name)
	return nil
}

// runSudo executes a command under sudo, inheriting the terminal so password prompts work.
func runSudo(ctx context.Context, args ...string) error {
	cmd := exec.CommandContext(ctx, "sudo", args...) //nolint:gosec // args are constructed internally, not from user input
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	logrus.Debugf("running: sudo %v", args)
	return cmd.Run()
}
