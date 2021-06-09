package main

import (
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

var pruneCommand = &cli.Command{
	Name:   "prune",
	Usage:  "Prune garbage objects",
	Action: pruneAction,
}

func pruneAction(clicontext *cli.Context) error {
	ucd, err := os.UserCacheDir()
	if err != nil {
		return err
	}
	cacheDir := filepath.Join(ucd, "lima")
	logrus.Infof("Pruning %q", cacheDir)
	return os.RemoveAll(cacheDir)
}
