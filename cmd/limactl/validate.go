package main

import (
	"bufio"
	"fmt"
	"io"
	"os"

	"github.com/lima-vm/lima/cmd/limactl/guessarg"
	"github.com/lima-vm/lima/pkg/cidata"
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
	return validateCommand
}

func firstLine(f string) (string, error) {
	file, err := os.Open(f)
	if err != nil {
		return "", err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	scanner.Scan()
	if err := scanner.Err(); err != nil {
		return "", err
	}
	return scanner.Text(), nil
}

func validateCloudConfig(f string) error {
	file, err := os.Open(f)
	if err != nil {
		return fmt.Errorf("failed to load YAML file %q: %w", f, err)
	}
	defer file.Close()
	b, err := io.ReadAll(file)
	if err != nil {
		return err
	}
	return cidata.ValidateCloudConfig(b)
}

func validateLimayaml(f string) error {
	_, err := store.LoadYAMLByFilePath(f)
	if err != nil {
		return fmt.Errorf("failed to load YAML file %q: %w", f, err)
	}
	if _, err := guessarg.InstNameFromYAMLPath(f); err != nil {
		return err
	}
	return nil
}

func validateAction(_ *cobra.Command, args []string) error {
	for _, f := range args {
		line, err := firstLine(f)
		if err != nil {
			return err
		}
		if line == "#cloud-config" {
			if err := validateCloudConfig(f); err != nil {
				return err
			}
		} else {
			if err := validateLimayaml(f); err != nil {
				return err
			}
		}
		logrus.Infof("%q: OK", f)
	}

	return nil
}
