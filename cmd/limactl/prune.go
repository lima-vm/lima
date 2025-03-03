// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"

	"github.com/lima-vm/lima/pkg/downloader"
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
	opt := downloader.WithCache()
	if !keepReferred {
		return downloader.RemoveAllCacheDir(opt)
	}

	// Prune downloads that are not used by any instances or templates
	cacheEntries, err := downloader.CacheEntries(opt)
	if err != nil {
		return err
	}
	knownLocations, err := knownLocations()
	if err != nil {
		return err
	}
	for cacheKey, cachePath := range cacheEntries {
		if file, exists := knownLocations[cacheKey]; exists {
			logrus.Debugf("Keep %q caching %q", cacheKey, file.Location)
		} else {
			logrus.Debug("Deleting ", cacheKey)
			if err := os.RemoveAll(cachePath); err != nil {
				logrus.Warnf("Failed to delete %q: %v", cacheKey, err)
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
		locations[downloader.CacheKey(f.Location)] = f.File
		if f.Kernel != nil {
			locations[downloader.CacheKey(f.Kernel.Location)] = f.Kernel.File
		}
		if f.Initrd != nil {
			locations[downloader.CacheKey(f.Initrd.Location)] = *f.Initrd
		}
	}
	for _, f := range y.Containerd.Archives {
		locations[downloader.CacheKey(f.Location)] = f
	}
	for _, f := range y.Firmware.Images {
		locations[downloader.CacheKey(f.Location)] = f.File
	}
	return locations
}
