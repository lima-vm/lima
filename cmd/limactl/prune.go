package main

import (
	"os"
	"path/filepath"

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
	ucd, err := os.UserCacheDir()
	if err != nil {
		return err
	}
	cacheDir := filepath.Join(ucd, "lima")
	logrus.Infof("Pruning %q", cacheDir)
	return os.RemoveAll(cacheDir)
}
