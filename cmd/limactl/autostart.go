// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"github.com/spf13/cobra"
)

func newAutostartCommand() *cobra.Command {
	autostartCommand := &cobra.Command{
		Use:     "autostart",
		Short:   "Manage automatic startup of Lima instances",
		GroupID: advancedCommand,
	}
	autostartCommand.AddCommand(newAutostartEnableCommand(), newAutostartDisableCommand())
	return autostartCommand
}

func newAutostartEnableCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "enable INSTANCE",
		Short:             "Register an instance to start automatically",
		Args:              WrapArgsError(cobra.ExactArgs(1)),
		RunE:              autostartEnableAction,
		ValidArgsFunction: autostartComplete,
	}
	flags := cmd.Flags()
	flags.String(
		"condition", "login",
		"When to start the instance: \"login\" (user session) or \"boot\" (system boot, macOS only)",
	)
	flags.String(
		"user", "",
		"macOS username to run the instance as when --condition=boot (default: $USER)",
	)
	return cmd
}

func newAutostartDisableCommand() *cobra.Command {
	return &cobra.Command{
		Use:               "disable INSTANCE",
		Short:             "Unregister an instance from automatic startup",
		Args:              WrapArgsError(cobra.ExactArgs(1)),
		RunE:              autostartDisableAction,
		ValidArgsFunction: autostartComplete,
	}
}

func autostartComplete(cmd *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	return bashCompleteInstanceNames(cmd)
}
