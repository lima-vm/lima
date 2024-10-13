package main

import (
	"fmt"

	"github.com/lima-vm/lima/cmd/limactl/guessarg"
	"github.com/lima-vm/lima/pkg/limayaml"
	"github.com/lima-vm/lima/pkg/store"
	"github.com/spf13/cobra"

	"github.com/sirupsen/logrus"
)

func newValidateCommand() *cobra.Command {
	validateCommand := &cobra.Command{
		Use:     "validate FILE.yaml [FILE.yaml, ...]",
		Short:   "Validate YAML files",
		Args:    WrapArgsError(cobra.MinimumNArgs(1)),
		RunE:    validateAction,
		GroupID: advancedCommand,
	}
	validateCommand.Flags().Bool("fill", false, "fill defaults")
	return validateCommand
}

func validateAction(cmd *cobra.Command, args []string) error {
	fill, err := cmd.Flags().GetBool("fill")
	if err != nil {
		return err
	}

	for _, f := range args {
		y, err := store.LoadYAMLByFilePath(f)
		if err != nil {
			return fmt.Errorf("failed to load YAML file %q: %w", f, err)
		}
		if _, err := guessarg.InstNameFromYAMLPath(f); err != nil {
			return err
		}
		logrus.Infof("%q: OK", f)
		if fill {
			b, err := limayaml.Marshal(y, len(args) > 1)
			if err != nil {
				return err
			}
			fmt.Fprint(cmd.OutOrStdout(), string(b))
		}
	}

	return nil
}
