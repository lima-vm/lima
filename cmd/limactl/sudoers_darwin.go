// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/sirupsen/logrus"
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
	if err := nwCfg.Validate(); err != nil {
		logrus.Infof("Please check %s for more information.", socketVMNetURL)
		return err
	}
	check, err := cmd.Flags().GetBool("check")
	if err != nil {
		return err
	}
	blockDevices, err := cmd.Flags().GetStringSlice("block-device")
	if err != nil {
		return err
	}
	if check {
		return verifySudoAccess(ctx, nwCfg, args, blockDevices, cmd.OutOrStdout())
	}
	switch len(args) {
	case 0:
		// NOP
	case 1:
		return errors.New("the file argument can be specified only for --check mode")
	default:
		return fmt.Errorf("unexpected arguments %v", args)
	}
	content, err := renderSudoers(nwCfg, blockDevices)
	if err != nil {
		return err
	}
	fmt.Fprint(cmd.OutOrStdout(), content)
	return nil
}

func renderSudoers(nwCfg networks.Config, blockDevices []string) (string, error) {
	networkSudoers, err := nwCfg.Sudoers()
	if err != nil {
		return "", err
	}
	var blockDeviceSudoers string
	if len(blockDevices) > 0 {
		blockDeviceSudoers, err = blockdevice.Sudoers(blockDevices)
		if err != nil {
			return "", err
		}
	}
	return sudoers.AssembleSudoersFragments(networkSudoers, blockDeviceSudoers), nil
}

func verifySudoAccess(ctx context.Context, nwCfg networks.Config, args, blockDevices []string, stdout io.Writer) error {
	var file string
	switch len(args) {
	case 0:
		file = nwCfg.Paths.Sudoers
		if file == "" {
			cfgFile, _ := networks.ConfigFile()
			return fmt.Errorf("no sudoers file defined in %#q", cfgFile)
		}
	case 1:
		file = args[0]
	default:
		return errors.New("can check only a single sudoers file")
	}
	if err := verifySudoersFile(ctx, nwCfg, file, blockDevices); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "%#q is up-to-date (or sudo doesn't require a password)\n", file)
	return nil
}

func verifySudoersFile(ctx context.Context, nwCfg networks.Config, file string, blockDevices []string) error {
	sudoersArgs := "sudoers"
	if len(blockDevices) > 0 {
		sudoersArgs += " --block-device=" + strings.Join(blockDevices, ",")
	}
	hint := fmt.Sprintf("run `%s %s >etc_sudoers.d_lima && sudo install -o root etc_sudoers.d_lima %q`)",
		os.Args[0], sudoersArgs, file)
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
	content, err := renderSudoers(nwCfg, blockDevices)
	if err != nil {
		return err
	}
	if string(b) != content {
		return fmt.Errorf("sudoers file %q is out of sync and must be regenerated (Hint: %s)", file, hint)
	}
	return nil
}
