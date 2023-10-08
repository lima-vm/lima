package main

import (
	"errors"
	"fmt"

	"github.com/lima-vm/lima/pkg/store"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func newUnprotectCommand() *cobra.Command {
	var unprotectCommand = &cobra.Command{
		Use:               "unprotect INSTANCE [INSTANCE, ...]",
		Short:             "Unprotect an instance",
		Args:              WrapArgsError(cobra.MinimumNArgs(1)),
		RunE:              unprotectAction,
		ValidArgsFunction: unprotectBashComplete,
	}
	return unprotectCommand
}

func unprotectAction(_ *cobra.Command, args []string) error {
	var errs []error
	for _, instName := range args {
		inst, err := store.Inspect(instName)
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
