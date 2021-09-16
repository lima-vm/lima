package main

import (
	"fmt"

	"github.com/lima-vm/lima/pkg/store"
	"github.com/spf13/cobra"

	"github.com/sirupsen/logrus"
)

func newValidateCommand() *cobra.Command {
	var validateCommand = &cobra.Command{
		Use:   "validate FILE.yaml [FILE.yaml, ...]",
		Short: "Validate YAML files",
		Args:  cobra.MinimumNArgs(1),
		RunE:  validateAction,
	}
	return validateCommand
}

func validateAction(cmd *cobra.Command, args []string) error {
	for _, f := range args {
		_, err := store.LoadYAMLByFilePath(f)
		if err != nil {
			return fmt.Errorf("failed to load YAML file %q: %w", f, err)
		}
		if _, err := instNameFromYAMLPath(f); err != nil {
			return err
		}
		logrus.Infof("%q: OK", f)
	}

	return nil
}
