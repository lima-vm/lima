/*
Copyright The Lima Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"errors"
	"fmt"

	"github.com/lima-vm/lima/pkg/store"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
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
