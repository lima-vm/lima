// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0
package main

import (
	"errors"
	"os"

	"github.com/lima-vm/lima/pkg/instance"
	networks "github.com/lima-vm/lima/pkg/networks/reconcile"
	"github.com/lima-vm/lima/pkg/store"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func newRestartCommand() *cobra.Command {
	restartCommand := &cobra.Command{
		Use:               "restart INSTANCE [INSTANCE, ...]",
		Aliases:           []string{},
		Short:             "Restarts an instance of Lima.",
		Args:              WrapArgsError(cobra.MinimumNArgs(1)),
		RunE:              restartAction,
		ValidArgsFunction: restartBashComplete,
		GroupID:           basicCommand,
	}

	restartCommand.Flags().BoolP("force", "f", false, "forcibly restarts the process")

	return restartCommand
}

func restartAction(cmd *cobra.Command, args []string) error {
	force, err := cmd.Flags().GetBool("force")
	if err != nil {
		return err
	}

	//stopping the instance
	for _, instName := range args {
		inst, err := store.Inspect(instName)

		switch inst.Status {
		case store.StatusStopped:
			logrus.Infof("The instance %q is already stopped. Run `limactl start` to start the instance.",
				inst.Name)
			// Not an error
			return nil
		case store.StatusRunning:
			// NOP
		default:
			logrus.Warnf("expected status %q, got %q", store.StatusStopped, inst.Status)
		}

		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				logrus.Warnf("Ignoring non-existent instance %q", instName)
				continue
			}
			return err
		}

		if force {
			instance.StopForcibly(inst)
		} else {
			err = instance.StopGracefully(inst)
		}
		// TODO: should we also reconcile networks if graceful stop returned an error?
		if err != nil {
			logrus.Warnf("Could not stop instance %q", instName)
			return err
		}
		ctx := cmd.Context()
		err = networks.Reconcile(ctx, inst.Name)
		if err != nil {
			return err
		}
		err = instance.Start(ctx, inst, "", false)
		if err != nil {
			return err
		}

		logrus.Infof("The instance %q restarted successfully.",inst.Name)
	}
	return err
}

func restartBashComplete(cmd *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	return bashCompleteDiskNames(cmd)
}
