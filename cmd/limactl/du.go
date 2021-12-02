package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/docker/go-units"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func newDiskUsageCommand() *cobra.Command {
	diskUsageCommand := &cobra.Command{
		Use:               "du",
		Short:             "Show lima disk usage",
		Args:              cobra.NoArgs,
		RunE:              diskUsageAction,
		ValidArgsFunction: cobra.NoFileCompletions,
	}
	return diskUsageCommand
}

func diskUsageAction(cmd *cobra.Command, args []string) error {
	return showCache()
}

var cache int64

func showCache() error {
	ucd, err := os.UserCacheDir()
	if err != nil {
		return err
	}
	cacheDir := filepath.Join(ucd, "lima")
	if err != nil {
		return err
	}

	logrus.Infof("Lima cache dir %s", cacheDir)
	cache = 0
	err = showCacheEntry(cacheDir)
	if err != nil {
		return err
	}
	logrus.Infof("%s", units.HumanSize(float64(cache)))
	return nil
}

var entry int64

func showCacheEntry(dir string) error {
	top := filepath.Join(dir, "download", "by-url-sha256")
	entry = 0
	err := filepath.WalkDir(top, showCacheDir)
	if err != nil {
		return err
	}
	return err
}

func showCacheDir(path string, d fs.DirEntry, err error) error {
	if !d.IsDir() {
		return nil
	}
	if d.Name() == "by-url-sha256" {
		return nil
	}
	dir := d.Name()
	urlFile := filepath.Join(path, "url")
	url, err := os.ReadFile(urlFile)
	if err != nil {
		return err
	}
	dataFile := filepath.Join(path, "data")
	st, err := os.Stat(dataFile)
	if err != nil {
		logrus.Warnf("%s", err)
		return nil
	}
	size := st.Size()
	fmt.Printf("%d\t%s\n", size/1024, url)
	entry += size
	logrus.Infof("%s %s", dir, units.HumanSize(float64(entry)))
	cache += entry
	return nil
}
