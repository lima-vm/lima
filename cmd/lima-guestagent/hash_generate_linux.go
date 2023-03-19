package main

import (
	"bytes"
	"fmt"
	"os"

	"github.com/reproducible-containers/repro-get/pkg/cache"
	"github.com/reproducible-containers/repro-get/pkg/distro"
	"github.com/reproducible-containers/repro-get/pkg/sha256sums"
	"github.com/spf13/cobra"
)

func newHashGenerateCommand() *cobra.Command {
	var hashGenerateCommand = &cobra.Command{
		Use:   "hash-generate",
		Short: "Generate the hash file (repro-get)",
		RunE:  hashGenerateAction,
	}
	hashGenerateCommand.Flags().String("dedupe", "", "Skip generating entries that are already presend in the specified file")
	return hashGenerateCommand
}

func hashGenerateAction(cmd *cobra.Command, args []string) error {
	d, err := getDistro(cmd)
	if err != nil {
		return err
	}

	ctx := cmd.Context()
	flags := cmd.Flags()

	opts := distro.HashOpts{
		FilterByName: args,
	}

	if d.Info().CacheIsNeededForGeneratingHash {
		cacheStr, err := flags.GetString("cache")
		if err != nil {
			return err
		}
		opts.Cache, err = cache.New(cacheStr)
		if err != nil {
			return err
		}
	}

	w := cmd.OutOrStdout()
	hw := distro.NewHashWriter(w)

	dedupeFile, err := flags.GetString("dedupe")
	if err != nil {
		return err
	}
	if dedupeFile != "" {
		old, err := os.ReadFile(dedupeFile)
		if err != nil {
			return fmt.Errorf("failed to open %q: %w", dedupeFile, err)
		}
		oldSums, err := sha256sums.Parse(bytes.NewReader(old))
		if err != nil {
			return fmt.Errorf("failed to parse %q as SHA256SUMS: %w", dedupeFile, err)
		}
		hw0 := hw
		hw = func(sha256sum, filename string) error {
			if oldSums[filename] == sha256sum {
				return nil
			}
			return hw0(sha256sum, filename)
		}
	}
	return d.GenerateHash(ctx, hw, opts)
}
