package main

import (
	"os"
	"path/filepath"

	"github.com/lima-vm/lima/pkg/downloader"
	"github.com/lima-vm/lima/pkg/limayaml"
	"github.com/lima-vm/lima/pkg/store"
	"github.com/opencontainers/go-digest"
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

	pruneCommand.Flags().Bool("dry-run", false, "Show which images would be deleted, but don't really delete them")
	pruneCommand.Flags().Bool("no-digest-only", false, "Only prune images without a digest specified (\"fallback\" images usually)")
	pruneCommand.Flags().Bool("unreferenced-only", false, "Only prune downloads not referenced in any VM")
	return pruneCommand
}

func deletePathMaybe(path string, isDryRun bool) error {
	if isDryRun {
		return nil
	}
	err := os.RemoveAll(path)
	if err != nil {
		logrus.WithError(err).WithFields(logrus.Fields{
			"path": path,
		}).Errorf("Cannot delete directory. Skipping...")
	}
	return err
}

func pruneAction(cmd *cobra.Command, args []string) error {
	pruneDryRun, err := cmd.Flags().GetBool("dry-run")
	if err != nil {
		return err
	}
	pruneWithoutDigest, err := cmd.Flags().GetBool("no-digest-only")
	if err != nil {
		return err
	}
	pruneUnreferenced, err := cmd.Flags().GetBool("unreferenced-only")
	if err != nil {
		return err
	}

	if pruneWithoutDigest || pruneUnreferenced {
		files, err := getReferencedDownloads()
		if err != nil {
			return err
		}

		cacheEntries, err := downloader.CachedDownloads(downloader.WithCache())
		if err != nil {
			return err
		}

		for _, entry := range cacheEntries {
			entryFields := logrus.Fields{
				"id":       entry.ID,
				"location": entry.Location,
			}

			logrus.WithFields(entryFields).Debug("cache entry")

			if pruneDryRun {
				entryFields["is_dry_run"] = pruneDryRun
			}

			// check if the cache entry is referenced
			if refFile, refFound := files[entry.ID]; refFound {
				if refFile.Location != entry.Location { // sanity check
					ef := entryFields
					ef["referenced_location"] = refFile.Location
					logrus.WithFields(ef).Warnf("Sanity check failed! URL mismatch")
				}

				if pruneWithoutDigest && refFile.Digest == "" {
					// delete the fallback image entry (entry w/o digest) even if referenced
					logrus.WithFields(entryFields).Infof("Deleting fallback entry")
					deletePathMaybe(entry.Path, pruneDryRun)
				}
			} else {
				if pruneUnreferenced {
					// delete the unreferenced cached entry
					logrus.WithFields(entryFields).Infof("Deleting unreferenced entry")
					deletePathMaybe(entry.Path, pruneDryRun)
				}
			}
		}
		return nil
	}

	// prune everything if no options specified
	ucd, err := os.UserCacheDir()
	if err != nil {
		return err
	}
	cacheDir := filepath.Join(ucd, "lima")
	logrus.WithFields(logrus.Fields{
		"is_dry_run": pruneDryRun,
		"location":   cacheDir,
	}).Infof("Pruning entire cache")
	return deletePathMaybe(cacheDir, pruneDryRun)
}

// Collect all downloads referenced in VM definitions and templates
func getReferencedDownloads() (map[string]limayaml.File, error) {
	digests := make(map[string]limayaml.File)

	vmRefs, err := store.Downloads()
	if err != nil {
		return nil, err
	}

	for _, f := range vmRefs {
		d := digest.SHA256.FromString(f.Location).Encoded()
		logrus.WithFields(logrus.Fields{
			"id":       d,
			"location": f.Location,
		}).Debugf("referenced file")
		digests[d] = f
	}
	return digests, nil
}
