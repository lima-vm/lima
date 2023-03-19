package main

import (
	"os"

	"github.com/lima-vm/lima/pkg/store/dirnames"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func newPruneCommand() *cobra.Command {
	pruneCommand := &cobra.Command{
		Use:               "prune",
		Short:             "Prune garbage objects",
		Args:              WrapArgsError(cobra.NoArgs),
		RunE:              pruneAction,
		ValidArgsFunction: cobra.NoFileCompletions,
	}
	return pruneCommand
}

func pruneAction(cmd *cobra.Command, args []string) error {
	cacheDir, err := dirnames.LimaCacheDir()
	if err != nil {
		return err
	}
	logrus.Infof("Pruning %q", cacheDir)
	return os.RemoveAll(cacheDir)
}
