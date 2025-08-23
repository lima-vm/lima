// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"github.com/spf13/cobra"

	"github.com/lima-vm/lima/v2/pkg/instance"
	"github.com/lima-vm/lima/v2/pkg/store"
)

func newRestartCommand() *cobra.Command {
	restartCmd := &cobra.Command{
		Use:               "restart INSTANCE",
		Short:             "Restart a running instance",
		Args:              WrapArgsError(cobra.MaximumNArgs(1)),
		RunE:              restartAction,
		ValidArgsFunction: restartBashComplete,
		GroupID:           basicCommand,
	}

	restartCmd.Flags().BoolP("force", "f", false, "Force stop and restart the instance")
	return restartCmd
}

func restartAction(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	instName := DefaultInstanceName
	if len(args) > 0 {
		instName = args[0]
	}

	inst, err := store.Inspect(ctx, instName)
	if err != nil {
		return err
	}

	force, err := cmd.Flags().GetBool("force")
	if err != nil {
		return err
	}

	if force {
		return instance.RestartForcibly(ctx, inst)
	}

	return instance.Restart(ctx, inst)
}

func restartBashComplete(cmd *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	return bashCompleteInstanceNames(cmd)
}
