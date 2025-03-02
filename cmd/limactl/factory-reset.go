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
	"os"
	"path/filepath"
	"strings"

	"github.com/lima-vm/lima/pkg/cidata"
	"github.com/lima-vm/lima/pkg/instance"
	"github.com/lima-vm/lima/pkg/store"
	"github.com/lima-vm/lima/pkg/store/filenames"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
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

func factoryResetAction(_ *cobra.Command, args []string) error {
	instName := DefaultInstanceName
	if len(args) > 0 {
		instName = args[0]
	}

	inst, err := store.Inspect(instName)
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
	if err := cidata.GenerateCloudConfig(inst.Dir, instName, inst.Config); err != nil {
		logrus.Error(err)
	}

	logrus.Infof("Instance %q has been factory reset", instName)
	return nil
}

func factoryResetBashComplete(cmd *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	return bashCompleteInstanceNames(cmd)
}
