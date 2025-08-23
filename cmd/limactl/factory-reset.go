// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/lima-vm/lima/v2/pkg/cidata"
	"github.com/lima-vm/lima/v2/pkg/instance"
	"github.com/lima-vm/lima/v2/pkg/limatype/filenames"
	"github.com/lima-vm/lima/v2/pkg/store"
)

func newFactoryResetCommand() *cobra.Command {
	resetCommand := &cobra.Command{
		Use:               "factory-reset INSTANCE",
		Short:             "Factory reset an instance of Lima",
		Args:              WrapArgsError(cobra.MaximumNArgs(1)),
		RunE:              factoryResetAction,
		ValidArgsFunction: factoryResetBashComplete,
		GroupID:           advancedCommand,
	}
	return resetCommand
}

func factoryResetAction(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	instName := DefaultInstanceName
	if len(args) > 0 {
		instName = args[0]
	}

	inst, err := store.Inspect(ctx, instName)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			logrus.Infof("Instance %q not found", instName)
			return nil
		}
		return err
	}
	if inst.Protected {
		return errors.New("instance is protected to prohibit accidental factory-reset (Hint: use `limactl unprotect`)")
	}

	instance.StopForcibly(inst)

	fi, err := os.ReadDir(inst.Dir)
	if err != nil {
		return err
	}
	retain := map[string]struct{}{
		filenames.LimaVersion:  {},
		filenames.Protected:    {},
		filenames.VzIdentifier: {},
	}
	for _, f := range fi {
		path := filepath.Join(inst.Dir, f.Name())
		if _, ok := retain[f.Name()]; !ok && !strings.HasSuffix(path, ".yaml") && !strings.HasSuffix(path, ".yml") {
			logrus.Infof("Removing %q", path)
			if err := os.Remove(path); err != nil {
				logrus.Error(err)
			}
		}
	}
	// Regenerate the cloud-config.yaml, to reflect any changes to the global _config
	if err := cidata.GenerateCloudConfig(ctx, inst.Dir, instName, inst.Config); err != nil {
		logrus.Error(err)
	}

	logrus.Infof("Instance %q has been factory reset", instName)
	return nil
}

func factoryResetBashComplete(cmd *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	return bashCompleteInstanceNames(cmd)
}
