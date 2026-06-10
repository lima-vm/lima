// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/user"
	"runtime"

	"github.com/spf13/cobra"

	"github.com/lima-vm/lima/v2/pkg/blockdevice"
	"github.com/lima-vm/lima/v2/pkg/networks"
	"github.com/lima-vm/lima/v2/pkg/sudoers"
)

const socketVMNetURL = "https://lima-vm.io/docs/config/network/vmnet/#socket_vmnet"

// newSudoersCommand is specific to macOS and Linux, but the help message is
// compiled on the other platforms too, as depended by `make docsy`.
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
		Long: fmt.Sprintf(`Generate the content of the /etc/sudoers.d/lima file for host helpers that require privilege escalation.
On macOS this includes vmnet.framework support (socket_vmnet) and host block-device attachment with --block-device on supported backends.
On other Unix hosts only the block-device helper is included; the entry is scoped to the current user.
Installing the file is optional: without it, attaching a block device that the user cannot open directly prompts for the sudo password once per start.
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
	return sudoersCommand
}

// sudoersContent assembles the per-OS sudoers file content. The fragments
// themselves are owned by their domain packages (pkg/networks for the vmnet
// helpers, pkg/blockdevice for the block-device helper); this only composes
// them.
func sudoersContent() (string, error) {
	switch runtime.GOOS {
	case "darwin":
		nwCfg, err := networks.LoadConfig()
		if err != nil {
			return "", err
		}
		networkSudoers, err := nwCfg.Sudoers()
		if err != nil {
			return "", err
		}
		blockDeviceSudoers, err := blockdevice.Sudoers("%" + nwCfg.Group)
		if err != nil {
			return "", err
		}
		return sudoers.AssembleSudoersFragments(networkSudoers, blockDeviceSudoers), nil
	case "windows":
		return "", errors.New("the sudoers command is not needed on Windows; run the Lima process elevated to attach block devices")
	default:
		// On Linux and the BSDs the only privileged helper is the
		// block-device one; the vmnet helpers are macOS-specific. The entry
		// is scoped to the invoking user instead of a group, because the
		// name of the admin group varies between distributions.
		u, err := user.Current()
		if err != nil {
			return "", err
		}
		blockDeviceSudoers, err := blockdevice.Sudoers(u.Username)
		if err != nil {
			return "", err
		}
		return sudoers.AssembleSudoersFragments(blockDeviceSudoers), nil
	}
}

// defaultSudoersFile is the path checked by `limactl sudoers --check` when no
// file argument is given.
func defaultSudoersFile() (string, error) {
	if runtime.GOOS == "darwin" {
		nwCfg, err := networks.LoadConfig()
		if err != nil {
			return "", err
		}
		if nwCfg.Paths.Sudoers == "" {
			cfgFile, _ := networks.ConfigFile()
			return "", fmt.Errorf("no sudoers file defined in %#q", cfgFile)
		}
		return nwCfg.Paths.Sudoers, nil
	}
	return "/etc/sudoers.d/lima", nil
}

func sudoersAction(cmd *cobra.Command, args []string) error {
	check, err := cmd.Flags().GetBool("check")
	if err != nil {
		return err
	}
	if check {
		return verifySudoAccess(cmd.Context(), args, cmd.OutOrStdout())
	}
	switch len(args) {
	case 0:
		// NOP
	case 1:
		return errors.New("the file argument can be specified only for --check mode")
	default:
		return fmt.Errorf("unexpected arguments %v", args)
	}
	content, err := sudoersContent()
	if err != nil {
		return err
	}
	fmt.Fprint(cmd.OutOrStdout(), content)
	return nil
}

func verifySudoAccess(ctx context.Context, args []string, stdout io.Writer) error {
	var file string
	switch len(args) {
	case 0:
		var err error
		file, err = defaultSudoersFile()
		if err != nil {
			return err
		}
	case 1:
		file = args[0]
	default:
		return errors.New("can check only a single sudoers file")
	}
	if err := verifySudoersFile(ctx, file); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "%#q is up-to-date (or sudo doesn't require a password)\n", file)
	return nil
}

func verifySudoersFile(ctx context.Context, file string) error {
	hint := fmt.Sprintf("run `%s sudoers >etc_sudoers.d_lima && sudo install -o root etc_sudoers.d_lima %q`)",
		os.Args[0], file)
	content, err := sudoersContent()
	if err != nil {
		return err
	}
	b, err := os.ReadFile(file)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) && runtime.GOOS == "darwin" {
			// The file is optional when sudo does not require a password.
			if nwCfg, cfgErr := networks.LoadConfig(); cfgErr == nil {
				if err := nwCfg.VerifySudoAccess(ctx, ""); err == nil {
					if err := sudoers.Run(ctx, "root", sudoers.RootGroup(), nil, nil, nil, "", "true"); err == nil {
						return nil
					}
				}
			}
		}
		return fmt.Errorf("can't read %q: %w: (Hint: %s)", file, err, hint)
	}
	if string(b) != content {
		return fmt.Errorf("sudoers file %q is out of sync and must be regenerated (Hint: %s)", file, hint)
	}
	return nil
}
