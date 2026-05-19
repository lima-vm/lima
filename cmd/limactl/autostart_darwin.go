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

	"github.com/lima-vm/lima/v2/pkg/autostart"
	"github.com/lima-vm/lima/v2/pkg/autostart/launchd"
	"github.com/lima-vm/lima/v2/pkg/store"
	"github.com/lima-vm/lima/v2/pkg/textutil"
)

func autostartEnableAction(cmd *cobra.Command, args []string) error {
	flags := cmd.Flags()
	condition, err := flags.GetString("condition")
	if err != nil {
		return err
	}
	keepAlive, err := flags.GetBool("keep-alive")
	if err != nil {
		return err
	}

	ctx := cmd.Context()
	inst, err := store.Inspect(ctx, args[0])
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("instance %q not found", args[0])
		}
		return err
	}

	switch condition {
	case "login":
		mgr := autostart.ManagerWith(keepAlive)
		if err := mgr.RegisterToStartAtLogin(ctx, inst); err != nil {
			return fmt.Errorf("failed to register instance %#q to start at login: %w", inst.Name, err)
		}
		logrus.Infof("Instance %#q registered to start at login", inst.Name)
	case "boot":
		userName, err := flags.GetString("user")
		if err != nil {
			return err
		}
		if userName == "" {
			userName = os.Getenv("USER")
		}
		if userName == "" {
			return errors.New("could not determine user; pass --user")
		}
		return daemonInstall(ctx, inst.Name, inst.Dir, userName, keepAlive)
	default:
		return fmt.Errorf("unknown condition %q: must be \"login\" or \"boot\"", condition)
	}
	return nil
}

func autostartDisableAction(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	inst, err := store.Inspect(ctx, args[0])
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("instance %q not found", args[0])
		}
		return err
	}

	// Check for a LaunchAgent (login) registration first.
	if registered, err := autostart.IsRegistered(ctx, inst); err != nil {
		return err
	} else if registered {
		if err := autostart.UnregisterFromStartAtLogin(ctx, inst); err != nil {
			return fmt.Errorf("failed to unregister instance %#q from start at login: %w", inst.Name, err)
		}
		logrus.Infof("Instance %#q unregistered from start at login", inst.Name)
		return nil
	}

	// Check for a LaunchDaemon (boot) installation.
	daemonPath := launchd.GetDaemonPlistPath(inst.Name)
	if _, err := os.Stat(daemonPath); err == nil {
		return daemonUninstall(ctx, inst.Name)
	}

	logrus.Infof("Instance %#q is not registered for automatic startup", inst.Name)
	return nil
}

func daemonInstall(ctx context.Context, instName, workDir, userName string, keepAlive bool) error {
	selfExe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("could not determine limactl path: %w", err)
	}

	vars := map[string]string{
		"Binary":   selfExe,
		"Instance": instName,
		"WorkDir":  workDir,
		"UserName": userName,
	}
	if keepAlive {
		vars["KeepAlive"] = "true"
	}
	content, err := textutil.ExecuteTemplate(launchd.DaemonTemplate, vars)
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

	destPath := launchd.GetDaemonPlistPath(instName)
	svcTarget := "system/" + launchd.DaemonServiceNameFrom(instName)

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

	logrus.Infof("LaunchDaemon installed for instance %q (runs as %q at boot)", instName, userName)
	logrus.Infof("Plist: %s", destPath)
	return nil
}

func daemonUninstall(ctx context.Context, instName string) error {
	svcTarget := "system/" + launchd.DaemonServiceNameFrom(instName)
	destPath := launchd.GetDaemonPlistPath(instName)

	_ = runSudo(ctx, "launchctl", "bootout", svcTarget)
	_ = runSudo(ctx, "launchctl", "disable", svcTarget)
	if err := runSudo(ctx, "rm", "-f", destPath); err != nil {
		return fmt.Errorf("failed to remove %s: %w", destPath, err)
	}
	logrus.Infof("LaunchDaemon uninstalled for instance %q", instName)
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
