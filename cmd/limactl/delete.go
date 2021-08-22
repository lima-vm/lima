package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/lima-vm/lima/pkg/store"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

var deleteCommand = &cli.Command{
	Name:      "delete",
	Aliases:   []string{"remove", "rm"},
	Usage:     "Delete an instance of Lima.",
	ArgsUsage: "INSTANCE [INSTANCE, ...]",
	Flags: []cli.Flag{
		&cli.BoolFlag{
			Name:    "force",
			Aliases: []string{"f"},
			Usage:   "forcibly kill the processes",
		},
	},
	Action:       deleteAction,
	BashComplete: deleteBashComplete,
}

func deleteAction(clicontext *cli.Context) error {
	if clicontext.NArg() == 0 {
		return fmt.Errorf("requires at least 1 argument")
	}
	force := clicontext.Bool("force")
	for _, instName := range clicontext.Args().Slice() {
		inst, err := store.Inspect(instName)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				logrus.Warnf("Ignoring non-existent instance %q", instName)
				continue
			}
			return err
		}
		if err := deleteInstance(inst, force); err != nil {
			return fmt.Errorf("failed to delete instance %q: %w", instName, err)
		}
		logrus.Infof("Deleted %q (%q)", instName, inst.Dir)
	}
	return nil
}

func deleteInstance(inst *store.Instance, force bool) error {
	if !force && inst.Status != store.StatusStopped {
		return fmt.Errorf("expected status %q, got %q", store.StatusStopped, inst.Status)
	}

	stopInstanceForcibly(inst)

	if err := os.RemoveAll(inst.Dir); err != nil {
		return fmt.Errorf("failed to remove %q: %w", inst.Dir, err)
	}
	return nil
}

func deleteBashComplete(clicontext *cli.Context) {
	bashCompleteInstanceNames(clicontext)
}
