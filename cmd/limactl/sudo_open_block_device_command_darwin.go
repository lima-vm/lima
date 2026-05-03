//go:build darwin

// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"github.com/spf13/cobra"

	"github.com/lima-vm/lima/v2/pkg/blockdevice"
)

func isPrivilegedHelperCommand(name string) bool {
	return name == blockdevice.SudoOpenBlockDeviceCommand
}

func registerHiddenCommands(rootCmd *cobra.Command) {
	rootCmd.AddCommand(&cobra.Command{
		Use:    blockdevice.SudoOpenBlockDeviceCommand,
		Short:  "Open a host block device via privileged helper",
		Args:   WrapArgsError(cobra.NoArgs),
		Hidden: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return blockdevice.ServeSudoOpenBlockDevice(cmd.InOrStdin())
		},
	})
}
