package main

import (
	"encoding/json"
	"fmt"

	"github.com/lima-vm/lima/pkg/infoutil"
	"github.com/spf13/cobra"
)

func newInfoCommand() *cobra.Command {
	infoCommand := &cobra.Command{
		Use:     "info",
		Short:   "Show diagnostic information",
		Args:    WrapArgsError(cobra.NoArgs),
		RunE:    infoAction,
		GroupID: "management",
	}
	return infoCommand
}

func infoAction(cmd *cobra.Command, _ []string) error {
	info, err := infoutil.GetInfo()
	if err != nil {
		return err
	}
	j, err := json.MarshalIndent(info, "", "    ")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(cmd.OutOrStdout(), string(j))
	return err
}
