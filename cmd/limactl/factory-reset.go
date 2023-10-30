package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/lima-vm/lima/pkg/store"
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

	stopInstanceForcibly(inst)

	fi, err := os.ReadDir(inst.Dir)
	if err != nil {
		return err
	}
	for _, f := range fi {
		path := filepath.Join(inst.Dir, f.Name())
		if !strings.HasSuffix(path, ".yaml") && !strings.HasSuffix(path, ".yml") {
			logrus.Infof("Removing %q", path)
			if err := os.Remove(path); err != nil {
				logrus.Error(err)
			}
		}
	}
	logrus.Infof("Instance %q has been factory reset", instName)
	return nil
}

func factoryResetBashComplete(cmd *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	return bashCompleteInstanceNames(cmd)
}
