// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"errors"

	"github.com/spf13/cobra"

	"github.com/lima-vm/lima/v2/pkg/instance"
	"github.com/lima-vm/lima/v2/pkg/networks/reconcile"
	"github.com/lima-vm/lima/v2/pkg/store"
)

func newStopCommand() *cobra.Command {
	stopCmd := &cobra.Command{
		Use:               "stop INSTANCE [INSTANCE, ...]",
		Short:             "Stop an instance",
		Args:              WrapArgsError(cobra.ArbitraryArgs),
		RunE:              stopAction,
		ValidArgsFunction: stopBashComplete,
		GroupID:           basicCommand,
	}

	stopCmd.Flags().BoolP("force", "f", false, "Force stop the instance")
	return stopCmd
}

func stopAction(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	instNames := args
	if len(instNames) == 0 {
		instNames = []string{DefaultInstanceName}
	}

	force, err := cmd.Flags().GetBool("force")
	if err != nil {
		return err
	}

	var errs []error
	for _, instName := range instNames {
		inst, err := store.Inspect(ctx, instName)
		if err != nil {
			errs = append(errs, err)
			continue
		}

		if force {
			instance.StopForcibly(inst)
		} else if err := instance.StopGracefully(ctx, inst, false); err != nil {
			errs = append(errs, err)
		}
	}

	// TODO: should we also reconcile networks if graceful stop returned an error?
	if err := reconcile.Reconcile(ctx, ""); err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

func stopBashComplete(cmd *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	return bashCompleteInstanceNames(cmd)
}
