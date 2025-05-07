// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"

	"github.com/lima-vm/lima/pkg/networks"
	"github.com/spf13/cobra"
)

const socketVMNetURL = "https://lima-vm.io/docs/config/network/vmnet/#socket_vmnet"

// newSudoersCommand is specific to macOS, but the help message is
// compiled on Linux too, as depended by `make docsy`.
// https://github.com/lima-vm/lima/issues/3436
func newSudoersCommand() *cobra.Command {
	sudoersCommand := &cobra.Command{
		Use: "sudoers [--check [SUDOERSFILE-TO-CHECK]]",
		Example: `
To generate the /etc/sudoers.d/lima file:
$ limactl sudoers | sudo tee /etc/sudoers.d/lima

To validate the existing /etc/sudoers.d/lima file:
$ limactl sudoers --check /etc/sudoers.d/lima
`,
		Short: "Generate the content of the /etc/sudoers.d/lima file",
		Long: fmt.Sprintf(`Generate the content of the /etc/sudoers.d/lima file for enabling vmnet.framework support (socket_vmnet) on macOS.
The content is written to stdout, NOT to the file.
This command must not run as the root user.
See %s for the usage.`, socketVMNetURL),
		Args:    WrapArgsError(cobra.MaximumNArgs(1)),
		RunE:    sudoersAction,
		GroupID: advancedCommand,
	}
	cfgFile, _ := networks.ConfigFile()
	sudoersCommand.Flags().Bool("check", false,
		fmt.Sprintf("check that the sudoers file is up-to-date with %q", cfgFile))
	return sudoersCommand
}
