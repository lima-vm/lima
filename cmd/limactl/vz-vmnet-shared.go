// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"github.com/spf13/cobra"
)

func newVzVmnetSharedCommand() *cobra.Command {
	newCommand := &cobra.Command{
		Use:               "vz-vmnet-shared",
		Short:             "Run vz-vmnet-shared",
		Args:              cobra.ExactArgs(0),
		RunE:              newVzVmnetSharedAction,
		ValidArgsFunction: newVzVmnetSharedComplete,
		Hidden:            true,
	}
	newCommand.Flags().Bool("enable-mach-service", false, "Enable Mach service")
	newCommand.Flags().String("mach-service", "", "Run as Mach service")
	_ = newCommand.Flags().MarkHidden("mach-service")
	return newCommand
}

func newVzVmnetSharedComplete(cmd *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	return bashCompleteInstanceNames(cmd)
}
