package main

import (
	"errors"
	"fmt"
	"runtime"

	"github.com/lima-vm/lima/pkg/networks"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

const networksURL = "https://lima-vm.io/docs/config/network/#socket_vmnet"

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
		Long: fmt.Sprintf(`Generate the content of the /etc/sudoers.d/lima file for enabling vmnet.framework support.
The content is written to stdout, NOT to the file.
This command must not run as the root user.
See %s for the usage.`, networksURL),
		Args:    WrapArgsError(cobra.MaximumNArgs(1)),
		RunE:    sudoersAction,
		GroupID: advancedCommand,
	}
	cfgFile, _ := networks.ConfigFile()
	sudoersCommand.Flags().Bool("check", false,
		fmt.Sprintf("check that the sudoers file is up-to-date with %q", cfgFile))
	return sudoersCommand
}

func sudoersAction(cmd *cobra.Command, args []string) error {
	if runtime.GOOS != "darwin" {
		return errors.New("sudoers command is only supported on macOS right now")
	}
	nwCfg, err := networks.LoadConfig()
	if err != nil {
		return err
	}
	// Make sure the current network configuration is secure
	if err := nwCfg.Validate(); err != nil {
		logrus.Infof("Please check %s for more information.", networksURL)
		return err
	}
	check, err := cmd.Flags().GetBool("check")
	if err != nil {
		return err
	}
	if check {
		return verifySudoAccess(nwCfg, args)
	}
	switch len(args) {
	case 0:
		// NOP
	case 1:
		return fmt.Errorf("the file argument can be specified only for --check mode")
	default:
		return fmt.Errorf("unexpected arguments %v", args)
	}
	sudoers, err := networks.Sudoers()
	if err != nil {
		return err
	}
	fmt.Print(sudoers)
	return nil
}

func verifySudoAccess(nwCfg networks.Config, args []string) error {
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
	fmt.Printf("%q is up-to-date (or sudo doesn't require a password)\n", file)
	return nil
}
