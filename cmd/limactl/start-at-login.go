// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"github.com/spf13/cobra"
)

func newStartAtLoginCommand() *cobra.Command {
	startAtLoginCommand := &cobra.Command{
		Use:               "start-at-login INSTANCE",
		Short:             "Register/Unregister an autostart file for the instance",
		Args:              WrapArgsError(cobra.MaximumNArgs(1)),
		RunE:              startAtLoginAction,
		ValidArgsFunction: startAtLoginComplete,
		GroupID:           advancedCommand,
	}

	startAtLoginCommand.Flags().Bool(
		"enabled", true,
		"Automatically start the instance when the user logs in",
	)

	return startAtLoginCommand
}

func startAtLoginComplete(cmd *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	return bashCompleteInstanceNames(cmd)
}
