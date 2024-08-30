package main

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"

	"github.com/lima-vm/lima/pkg/limayaml"
	"github.com/lima-vm/lima/pkg/store"
	"github.com/lima-vm/lima/pkg/templatestore"
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
		GroupID:           advancedCommand,
	}
	pruneCommand.Flags().Bool("keep-referred", false, "Keep objects that are referred by some instances or templates")
	return pruneCommand
}

func pruneAction(cmd *cobra.Command, _ []string) error {
	keepReferred, err := cmd.Flags().GetBool("keep-referred")
	if err != nil {
		return err
	}
	ucd, err := os.UserCacheDir()
	if err != nil {
		return err
	}
	cacheDir := filepath.Join(ucd, "lima")
	logrus.Infof("Pruning %q", cacheDir)
	if !keepReferred {
		return os.RemoveAll(cacheDir)
	}

	// Prune downloads that are not used by any instances or templates
	downloadDir := filepath.Join(cacheDir, "download", "by-url-sha256")
	_, err = os.Stat(downloadDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	cacheEntries, err := os.ReadDir(downloadDir)
	if err != nil {
		return err
	}
	knownLocations, err := knownLocations()
	if err != nil {
		return err
	}
	for _, entry := range cacheEntries {
		if file, exists := knownLocations[entry.Name()]; exists {
			logrus.Debugf("Keep %q caching %q", entry.Name(), file.Location)
		} else {
			logrus.Debug("Deleting ", entry.Name())
			if err := os.RemoveAll(filepath.Join(downloadDir, entry.Name())); err != nil {
				logrus.Warnf("Failed to delete %q: %v", entry.Name(), err)
				return err
			}
		}
	}
	return nil
}

func knownLocations() (map[string]limayaml.File, error) {
	locations := make(map[string]limayaml.File)

	// Collect locations from instances
	instances, err := store.Instances()
	if err != nil {
		return nil, err
	}
	for _, instanceName := range instances {
		instance, err := store.Inspect(instanceName)
		if err != nil {
			return nil, err
		}
		for k, v := range locationsFromLimaYAML(instance.Config) {
			locations[k] = v
		}
	}

	// Collect locations from templates
	templates, err := templatestore.Templates()
	if err != nil {
		return nil, err
	}
	for _, t := range templates {
		b, err := templatestore.Read(t.Name)
		if err != nil {
			return nil, err
		}
		y, err := limayaml.Load(b, t.Name)
		if err != nil {
			return nil, err
		}
		for k, v := range locationsFromLimaYAML(y) {
			locations[k] = v
		}
	}
	return locations, nil
}

func locationsFromLimaYAML(y *limayaml.LimaYAML) map[string]limayaml.File {
	locations := make(map[string]limayaml.File)
	for _, f := range y.Images {
		locations[sha256OfURL(f.Location)] = f.File
		if f.Kernel != nil {
			locations[sha256OfURL(f.Kernel.Location)] = f.Kernel.File
		}
		if f.Initrd != nil {
			locations[sha256OfURL(f.Initrd.Location)] = *f.Initrd
		}
	}
	for _, f := range y.Containerd.Archives {
		locations[sha256OfURL(f.Location)] = f
	}
	for _, f := range y.Firmware.Images {
		locations[sha256OfURL(f.Location)] = f.File
	}
	return locations
}

func sha256OfURL(url string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(url)))
}
