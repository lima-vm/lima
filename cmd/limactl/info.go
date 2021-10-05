package main

import (
	"encoding/json"
	"fmt"

	"github.com/lima-vm/lima/pkg/limayaml"
	"github.com/lima-vm/lima/pkg/version"
	"github.com/spf13/cobra"
)

func newInfoCommand() *cobra.Command {
	infoCommand := &cobra.Command{
		Use:   "info",
		Short: "Show diagnostic information",
		Args:  cobra.NoArgs,
		RunE:  infoAction,
	}
	return infoCommand
}

type Info struct {
	Version         string             `json:"version"`
	DefaultTemplate *limayaml.LimaYAML `json:"defaultTemplate"`
	// TODO: add diagnostic info of QEMU
}

func infoAction(cmd *cobra.Command, args []string) error {
	y, err := limayaml.Load(limayaml.DefaultTemplate, "")
	if err != nil {
		return err
	}
	info := &Info{
		Version:         version.Version,
		DefaultTemplate: y,
	}
	j, err := json.MarshalIndent(info, "", "    ")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(cmd.OutOrStdout(), string(j))
	return err
}
