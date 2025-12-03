// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"github.com/spf13/cobra"
)

func newVmnetCommand() *cobra.Command {
	newCommand := &cobra.Command{
		Use:               "vmnet",
		Short:             "Run vmnet",
		Args:              cobra.ExactArgs(0),
		RunE:              newVmnetAction,
		ValidArgsFunction: newVmnetComplete,
		Hidden:            true,
	}
	newCommand.Flags().Bool("unregister-mach-service", false, "Unregister Mach service")
	newCommand.Flags().String("mach-service", "", "Run as Mach service")
	_ = newCommand.Flags().MarkHidden("mach-service")
	return newCommand
}

func newVmnetComplete(cmd *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	return bashCompleteInstanceNames(cmd)
}
