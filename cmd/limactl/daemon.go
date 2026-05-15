// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"github.com/spf13/cobra"
)

func newDaemonCommand() *cobra.Command {
	daemonCommand := &cobra.Command{
		Use:     "daemon",
		Short:   "Manage Lima instances as system LaunchDaemons (macOS only, requires root)",
		GroupID: advancedCommand,
	}
	daemonCommand.AddCommand(newDaemonInstallCommand(), newDaemonUninstallCommand())
	return daemonCommand
}

func newDaemonInstallCommand() *cobra.Command {
	installCommand := &cobra.Command{
		Use:               "install INSTANCE",
		Short:             "Install a system LaunchDaemon for the instance (run with sudo)",
		Args:              WrapArgsError(cobra.ExactArgs(1)),
		RunE:              daemonInstallAction,
		ValidArgsFunction: daemonComplete,
	}
	installCommand.Flags().String(
		"user", "",
		"macOS username to run the daemon as (default: $USER)",
	)
	return installCommand
}

func newDaemonUninstallCommand() *cobra.Command {
	return &cobra.Command{
		Use:               "uninstall INSTANCE",
		Short:             "Uninstall the system LaunchDaemon for the instance (run with sudo)",
		Args:              WrapArgsError(cobra.ExactArgs(1)),
		RunE:              daemonUninstallAction,
		ValidArgsFunction: daemonComplete,
	}
}

func daemonComplete(cmd *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	return bashCompleteInstanceNames(cmd)
}
