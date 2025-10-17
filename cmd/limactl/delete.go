// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/lima-vm/lima/v2/pkg/autostart"
	"github.com/lima-vm/lima/v2/pkg/instance"
	"github.com/lima-vm/lima/v2/pkg/networks/reconcile"
	"github.com/lima-vm/lima/v2/pkg/store"
)

func newDeleteCommand() *cobra.Command {
	deleteCommand := &cobra.Command{
		Use:               "delete INSTANCE [INSTANCE, ...]",
		Aliases:           []string{"remove", "rm"},
		Short:             "Delete an instance of Lima",
		Args:              WrapArgsError(cobra.MinimumNArgs(1)),
		RunE:              deleteAction,
		ValidArgsFunction: deleteBashComplete,
		GroupID:           basicCommand,
	}
	deleteCommand.Flags().BoolP("force", "f", false, "Forcibly kill the processes")
	return deleteCommand
}

func deleteAction(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	force, err := cmd.Flags().GetBool("force")
	if err != nil {
		return err
	}
	for _, instName := range args {
		inst, err := store.Inspect(ctx, instName)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				logrus.Warnf("Ignoring non-existent instance %q", instName)
				continue
			}
			return err
		}
		if err := instance.Delete(cmd.Context(), inst, force); err != nil {
			return fmt.Errorf("failed to delete instance %q: %w", instName, err)
		}
		if registered, err := autostart.IsRegistered(ctx, inst); err != nil && !errors.Is(err, autostart.ErrNotSupported) {
			logrus.WithError(err).Warnf("Failed to check if the autostart entry for instance %q is registered", instName)
		} else if registered {
			if err := autostart.UnregisterFromStartAtLogin(ctx, inst); err != nil {
				logrus.WithError(err).Warnf("Failed to unregister the autostart entry for instance %q", instName)
			} else {
				logrus.Infof("The autostart entry for instance %q has been unregistered", instName)
			}
		} else {
			logrus.Infof("The autostart entry for instance %q is not registered", instName)
		}
		logrus.Infof("Deleted %q (%q)", instName, inst.Dir)
	}
	return reconcile.Reconcile(cmd.Context(), "")
}

func deleteBashComplete(cmd *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	return bashCompleteInstanceNames(cmd)
}
