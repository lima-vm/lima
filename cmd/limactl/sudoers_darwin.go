// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"errors"
	"fmt"
	"io"

	"github.com/lima-vm/lima/pkg/networks"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func sudoersAction(cmd *cobra.Command, args []string) error {
	nwCfg, err := networks.LoadConfig()
	if err != nil {
		return err
	}
	// Make sure the current network configuration is secure
	if err := nwCfg.Validate(); err != nil {
		logrus.Infof("Please check %s for more information.", socketVMNetURL)
		return err
	}
	check, err := cmd.Flags().GetBool("check")
	if err != nil {
		return err
	}
	if check {
		return verifySudoAccess(nwCfg, args, cmd.OutOrStdout())
	}
	switch len(args) {
	case 0:
		// NOP
	case 1:
		return errors.New("the file argument can be specified only for --check mode")
	default:
		return fmt.Errorf("unexpected arguments %v", args)
	}
	sudoers, err := networks.Sudoers()
	if err != nil {
		return err
	}
	fmt.Fprint(cmd.OutOrStdout(), sudoers)
	return nil
}

func verifySudoAccess(nwCfg networks.Config, args []string, stdout io.Writer) error {
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
	if err := nwCfg.VerifySudoAccess(file); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "%q is up-to-date (or sudo doesn't require a password)\n", file)
	return nil
}
