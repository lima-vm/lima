// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/lima-vm/lima/v2/pkg/copytool"
)

const copyHelp = `Copy files between host and guest

Prefix guest filenames with the instance name and a colon.

Backends:
  auto   - Automatically selects the best available backend (rsync preferred, falls back to scp)
  rsync  - Uses rsync for faster transfers with resume capability (requires rsync on both host and guest)
  scp    - Uses scp for reliable transfers (always available)

Not to be confused with 'limactl clone'.
`

const copyExample = `
  # Copy file from guest to host (auto backend)
  limactl copy default:/etc/os-release .

  # Copy file from host to guest with verbose output
  limactl copy -v myfile.txt default:/tmp/

  # Copy directory recursively using rsync backend
  limactl copy --backend=rsync -r ./mydir default:/tmp/

  # Copy using scp backend specifically
  limactl copy --backend=scp default:/var/log/app.log ./logs/

  # Copy multiple files
  limactl copy file1.txt file2.txt default:/tmp/
`

func newCopyCommand() *cobra.Command {
	copyCommand := &cobra.Command{
		Use:     "copy SOURCE ... TARGET",
		Aliases: []string{"cp"},
		Short:   "Copy files between host and guest",
		Long:    copyHelp,
		Example: copyExample,
		Args:    WrapArgsError(cobra.MinimumNArgs(2)),
		RunE:    copyAction,
		GroupID: advancedCommand,
	}

	copyCommand.Flags().BoolP("recursive", "r", false, "Copy directories recursively")
	copyCommand.Flags().BoolP("verbose", "v", false, "Enable verbose output")
	copyCommand.Flags().String("backend", "auto", "Copy backend (scp|rsync|auto)")

	return copyCommand
}

func copyAction(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	recursive, err := cmd.Flags().GetBool("recursive")
	if err != nil {
		return err
	}

	verbose, err := cmd.Flags().GetBool("verbose")
	if err != nil {
		return err
	}

	debug, err := cmd.Flags().GetBool("debug")
	if err != nil {
		return err
	}

	if debug {
		verbose = true
	}

	backend, err := cmd.Flags().GetString("backend")
	if err != nil {
		return err
	}

	cpTool, err := copytool.New(ctx, backend, args, &copytool.Options{
		Recursive: recursive,
		Verbose:   verbose,
	})
	if err != nil {
		return err
	}
	logrus.Debugf("using copy tool %q", cpTool.Name())

	copyCmd, err := cpTool.Command(ctx, args, nil)
	if err != nil {
		return err
	}

	copyCmd.Stdin = cmd.InOrStdin()
	copyCmd.Stdout = cmd.OutOrStdout()
	copyCmd.Stderr = cmd.ErrOrStderr()
	logrus.Debugf("executing %v (may take a long time)", copyCmd)

	// TODO: use syscall.Exec directly (results in losing tty?)
	return copyCmd.Run()
}
