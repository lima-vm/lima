package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/lima-vm/lima/pkg/networks"
	"github.com/spf13/cobra"
)

func newSudoersCommand() *cobra.Command {
	networksMD := "$PREFIX/share/doc/lima/docs/network.md"
	if exe, err := os.Executable(); err == nil {
		binDir := filepath.Dir(exe)
		prefixDir := filepath.Dir(binDir)
		networksMD = filepath.Join(prefixDir, "share/doc/lima/docs/network.md")
	}
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
This command must not run as the root.
See %s for the usage.`, networksMD),
		Args: WrapArgsError(cobra.MaximumNArgs(1)),
		RunE: sudoersAction,
	}
	configFile, _ := networks.ConfigFile()
	sudoersCommand.Flags().Bool("check", false,
		fmt.Sprintf("check that the sudoers file is up-to-date with %q", configFile))
	return sudoersCommand
}

func sudoersAction(cmd *cobra.Command, args []string) error {
	if runtime.GOOS != "darwin" {
		return errors.New("sudoers command is only supported on macOS right now")
	}
	check, err := cmd.Flags().GetBool("check")
	if err != nil {
		return err
	}
	if check {
		return verifySudoAccess(args)
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

func verifySudoAccess(args []string) error {
	config, err := networks.Config()
	if err != nil {
		return err
	}
	if err := config.Validate(); err != nil {
		return err
	}
	var file string
	switch len(args) {
	case 0:
		file = config.Paths.Sudoers
		if file == "" {
			configFile, _ := networks.ConfigFile()
			return fmt.Errorf("no sudoers file defined in %q", configFile)
		}
	case 1:
		file = args[0]
	default:
		return errors.New("can check only a single sudoers file")
	}
	if err := config.VerifySudoAccess(file); err != nil {
		return err
	}
	fmt.Printf("%q is up-to-date (or sudo doesn't require a password)\n", file)
	return nil
}
