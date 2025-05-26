// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"errors"
	"fmt"
	"os"
	"runtime"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/lima-vm/lima/pkg/autostart"
	"github.com/lima-vm/lima/pkg/instance"
	networks "github.com/lima-vm/lima/pkg/networks/reconcile"
	"github.com/lima-vm/lima/pkg/store"
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
	force, err := cmd.Flags().GetBool("force")
	if err != nil {
		return err
	}
	for _, instName := range args {
		inst, err := store.Inspect(instName)
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
		if runtime.GOOS == "darwin" || runtime.GOOS == "linux" {
			deleted, err := autostart.DeleteStartAtLoginEntry(runtime.GOOS, instName)
			if err != nil && !errors.Is(err, os.ErrNotExist) {
				logrus.WithError(err).Warnf("The autostart file for instance %q does not exist", instName)
			} else if deleted {
				logrus.Infof("The autostart file %q has been deleted", autostart.GetFilePath(runtime.GOOS, instName))
			}
		}
		logrus.Infof("Deleted %q (%q)", instName, inst.Dir)
	}
	return networks.Reconcile(cmd.Context(), "")
}

func deleteBashComplete(cmd *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	return bashCompleteInstanceNames(cmd)
}
