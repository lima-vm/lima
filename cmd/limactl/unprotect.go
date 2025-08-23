// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"errors"
	"fmt"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/lima-vm/lima/v2/pkg/store"
)

func newUnprotectCommand() *cobra.Command {
	unprotectCommand := &cobra.Command{
		Use:               "unprotect INSTANCE [INSTANCE, ...]",
		Short:             "Unprotect an instance",
		Args:              WrapArgsError(cobra.MinimumNArgs(1)),
		RunE:              unprotectAction,
		ValidArgsFunction: unprotectBashComplete,
		GroupID:           advancedCommand,
	}
	return unprotectCommand
}

func unprotectAction(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	var errs []error
	for _, instName := range args {
		inst, err := store.Inspect(ctx, instName)
		if err != nil {
			errs = append(errs, fmt.Errorf("failed to inspect instance %q: %w", instName, err))
			continue
		}
		if !inst.Protected {
			logrus.Warnf("Instance %q isn't protected. Skipping.", instName)
			continue
		}
		if err := inst.Unprotect(); err != nil {
			errs = append(errs, fmt.Errorf("failed to unprotect instance %q: %w", instName, err))
			continue
		}
		logrus.Infof("Unprotected %q", instName)
	}
	return errors.Join(errs...)
}

func unprotectBashComplete(cmd *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	return bashCompleteInstanceNames(cmd)
}
