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

func newProtectCommand() *cobra.Command {
	protectCommand := &cobra.Command{
		Use:   "protect INSTANCE [INSTANCE, ...]",
		Short: "Protect an instance to prohibit accidental removal",
		Long: `Protect an instance to prohibit accidental removal via the 'limactl delete' command.
The instance is not being protected against removal via '/bin/rm', Finder, etc.`,
		Args:              WrapArgsError(cobra.MinimumNArgs(1)),
		RunE:              protectAction,
		ValidArgsFunction: protectBashComplete,
		GroupID:           advancedCommand,
	}
	return protectCommand
}

func protectAction(_ *cobra.Command, args []string) error {
	var errs []error
	for _, instName := range args {
		inst, err := store.Inspect(instName)
		if err != nil {
			errs = append(errs, fmt.Errorf("failed to inspect instance %q: %w", instName, err))
			continue
		}
		if inst.Protected {
			logrus.Warnf("Instance %q is already protected. Skipping.", instName)
			continue
		}
		if err := inst.Protect(); err != nil {
			errs = append(errs, fmt.Errorf("failed to protect instance %q: %w", instName, err))
			continue
		}
		logrus.Infof("Protected %q", instName)
	}
	return errors.Join(errs...)
}

func protectBashComplete(cmd *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	return bashCompleteInstanceNames(cmd)
}
