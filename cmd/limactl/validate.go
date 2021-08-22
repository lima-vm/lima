package main

import (
	"fmt"

	"github.com/lima-vm/lima/pkg/store"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

var validateCommand = &cli.Command{
	Name:      "validate",
	Usage:     "Validate yaml files",
	ArgsUsage: "FILE.yaml [FILE.yaml, ...]",
	Action:    validateAction,
}

func validateAction(clicontext *cli.Context) error {
	if clicontext.NArg() == 0 {
		return fmt.Errorf("requires at least 1 argument")
	}

	for _, f := range clicontext.Args().Slice() {
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
