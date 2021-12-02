package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/docker/go-units"
	"github.com/lima-vm/lima/pkg/store"
	"github.com/lima-vm/lima/pkg/store/dirnames"
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
	diskUsageCommand.Flags().Bool("cache", false, "Show cache usage")
	return diskUsageCommand
}

func diskUsageAction(cmd *cobra.Command, args []string) error {
	cache, err := cmd.Flags().GetBool("cache")
	if err != nil {
		return err
	}

	if !cache {
		return showHome()
	} else {
		return showCache()
	}
}

var total int64

func showHome() error {
	homeDir, err := dirnames.LimaDir()
	if err != nil {
		return err
	}
	logrus.Infof("Lima home dir %s", homeDir)
	total = 0
	instances, err := store.Instances()
	if err != nil {
		return err
	}
	for _, instance := range instances {
		if err = showInstance(instance); err != nil {
			return err
		}
	}
	logrus.Infof("%s", units.HumanSize(float64(total)))
	return nil
}

var instance int64

func showInstance(name string) error {
	dir, err := store.InstanceDir(name)
	if err != nil {
		return err
	}
	instance = 0
	err = filepath.WalkDir(dir, showInstanceDir)
	if err != nil {
		return err
	}
	logrus.Infof("%s %s", name, units.HumanSize(float64(instance)))
	total += instance
	return nil
}

func showInstanceDir(path string, d fs.DirEntry, err error) error {
	if d.IsDir() {
		return nil
	}
	if strings.HasSuffix(d.Name(), ".sock") {
		return nil
	}
	info, err := d.Info()
	if err != nil {
		logrus.Warnf("%s", err)
		return nil
	}
	size := info.Size()
	fmt.Printf("%d\t%s\n", size/1024, path)
	instance += size
	return nil
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
