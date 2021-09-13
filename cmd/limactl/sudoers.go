package main

import (
	"errors"
	"fmt"
	"github.com/lima-vm/lima/pkg/networks"
	"github.com/spf13/cobra"
)

func newSudoersCommand() *cobra.Command {
	sudoersCommand := &cobra.Command{
		Use:               "sudoers [SUDOERSFILE]",
		Short:             "Generate /etc/sudoers.d/lima file.",
		Args:              cobra.MaximumNArgs(1),
		RunE:              sudoersAction,
	}
	sudoersCommand.Flags().Bool("check", false,
		"check that the sudoers file is up-to-date with $LIMA_HOME/_config/networks.yaml")
	return sudoersCommand
}

func sudoersAction(cmd *cobra.Command, args []string) error {
	check, err := cmd.Flags().GetBool("check")
	if err != nil {
		return err
	}
	if check	{
		var file string
		switch len(args) {
		case 0:
			config, err := networks.Config()
			if err != nil {
				return err
			}
			file = config.Paths.Sudoers
			if file == "" {
				return errors.New("no sudoers file defined in ~/.lima/_config/networks.yaml")
			}
		case 1:
			file = args[0]
		default:
			return errors.New("can check only a single sudoers file")
		}
		if err := networks.CheckSudoers(file); err != nil {
			return err
		}
		fmt.Printf("%q is up-to-date\n", file)
		return nil
	}
	sudoers, err := networks.Sudoers()
	if err != nil {
		return err
	}
	fmt.Print(sudoers)
	return nil
}
