// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/lima-vm/lima/v2/pkg/blockdevice"
	"github.com/lima-vm/lima/v2/pkg/networks"
	"github.com/lima-vm/lima/v2/pkg/sudoers"
)

func sudoersAction(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	nwCfg, err := networks.LoadConfig()
	if err != nil {
		return err
	}
	check, err := cmd.Flags().GetBool("check")
	if err != nil {
		return err
	}
	if check {
		return verifySudoAccess(ctx, nwCfg, args, cmd.OutOrStdout())
	}
	switch len(args) {
	case 0:
		// NOP
	case 1:
		return errors.New("the file argument can be specified only for --check mode")
	default:
		return fmt.Errorf("unexpected arguments %v", args)
	}
	networkSudoers, err := nwCfg.Sudoers()
	if err != nil {
		return err
	}
	blockDeviceSudoers, err := blockdevice.Sudoers(nwCfg.Group)
	if err != nil {
		return err
	}
	content := sudoers.AssembleSudoersFragments(networkSudoers, blockDeviceSudoers)
	fmt.Fprint(cmd.OutOrStdout(), content)
	return nil
}

func verifySudoAccess(ctx context.Context, nwCfg networks.Config, args []string, stdout io.Writer) error {
	var file string
	switch len(args) {
	case 0:
		file = nwCfg.Paths.Sudoers
		if file == "" {
			cfgFile, _ := networks.ConfigFile()
			return fmt.Errorf("no sudoers file defined in %q", cfgFile)
		}
	case 1:
		file = args[0]
	default:
		return errors.New("can check only a single sudoers file")
	}
	if err := verifySudoersFile(ctx, nwCfg, file); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "%q is up-to-date (or sudo doesn't require a password)\n", file)
	return nil
}

func verifySudoersFile(ctx context.Context, nwCfg networks.Config, file string) error {
	hint := fmt.Sprintf("run `%s sudoers >etc_sudoers.d_lima && sudo install -o root etc_sudoers.d_lima %q`)",
		os.Args[0], file)
	b, err := os.ReadFile(file)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			if err := nwCfg.VerifySudoAccess(ctx, ""); err == nil {
				if err := sudoers.Run(ctx, "root", "wheel", nil, nil, nil, "", "true"); err == nil {
					return nil
				}
			}
		}
		return fmt.Errorf("can't read %q: %w: (Hint: %s)", file, err, hint)
	}
	networkSudoers, err := nwCfg.Sudoers()
	if err != nil {
		return err
	}
	blockDeviceSudoers, err := blockdevice.Sudoers(nwCfg.Group)
	if err != nil {
		return err
	}
	content := sudoers.AssembleSudoersFragments(networkSudoers, blockDeviceSudoers)
	if string(b) != content {
		return fmt.Errorf("sudoers file %q is out of sync and must be regenerated (Hint: %s)", file, hint)
	}
	return nil
}
