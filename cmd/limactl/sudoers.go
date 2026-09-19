// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/lima-vm/lima/v2/pkg/networks"
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

To authorize a host block device (include every device grant you want to keep):
$ limactl sudoers --block-device=/dev/disk4 >etc_sudoers.d_lima
$ less etc_sudoers.d_lima
$ visudo -cf etc_sudoers.d_lima
$ sudo install -o root -g wheel -m 0440 etc_sudoers.d_lima /etc/sudoers.d/lima
`,
		Short: "Generate the content of the /etc/sudoers.d/lima file",
		Long: fmt.Sprintf(`Generate the content of the /etc/sudoers.d/lima file for macOS host helpers that require privilege escalation.
This includes vmnet.framework support (socket_vmnet). Use --block-device=/dev/rdiskN to also emit opt-in
host block-device helper entries for the listed devices and current user.
Block devices require a root-owned helper and ancestor directories; user-writable
installations need additional setup: https://lima-vm.io/docs/config/disk/#sudoers-setup
--block-device on start/edit selects devices; it does not install sudoers grants.
The content is written to stdout, NOT to the file.
This command must not run as the root user.
See %s for the usage.`, socketVMNetURL),
		Args:    WrapArgsError(cobra.MaximumNArgs(1)),
		RunE:    sudoersAction,
		GroupID: advancedCommand,
	}
	cfgFile, _ := networks.ConfigFile()
	sudoersCommand.Flags().Bool("check", false,
		fmt.Sprintf("check that the sudoers file is up-to-date with %#q", cfgFile))
	sudoersCommand.Flags().StringSlice("block-device", nil,
		"include the macOS VZ host block-device helper for the current user and comma-separated devices")
	return sudoersCommand
}
