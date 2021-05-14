package main

import (
	"os"

	"github.com/AkihiroSuda/lima/pkg/store"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

var deleteCommand = &cli.Command{
	Name:         "delete",
	Usage:        "Delete an instance of Lima.",
	ArgsUsage:    "INSTANCE [INSTANCE, ...]",
	Action:       deleteAction,
	BashComplete: deleteBashComplete,
}

func deleteAction(clicontext *cli.Context) error {
	if clicontext.NArg() == 0 {
		return errors.Errorf("requires at least 1 argument")
	}
	for _, instName := range clicontext.Args().Slice() {
		instDir, err := store.InstanceDir(instName)
		if err != nil {
			return err
		}
		if _, err := os.Stat(instDir); errors.Is(err, os.ErrNotExist) {
			logrus.Warnf("Ignoring non-existent instance %q (%q)", instName, instDir)
			return nil
		}
		if err := os.RemoveAll(instDir); err != nil {
			return errors.Wrapf(err, "failed to remove %q", instDir)
		}
		logrus.Infof("Deleted %q (%q)", instName, instDir)
	}
	return nil
}

func deleteBashComplete(clicontext *cli.Context) {
	bashCompleteInstanceNames(clicontext)
}
