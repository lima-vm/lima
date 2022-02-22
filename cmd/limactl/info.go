package main

import (
	"encoding/json"
	"fmt"

	"github.com/lima-vm/lima/pkg/limayaml"
	"github.com/lima-vm/lima/pkg/store/dirnames"
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
	LimaHome        string             `json:"limaHome"`
	// TODO: add diagnostic info of QEMU
}

func infoAction(cmd *cobra.Command, args []string) error {
	b, err := readDefaultTemplate()
	if err != nil {
		return err
	}
	y, err := limayaml.Load(b, "")
	if err != nil {
		return err
	}
	info := &Info{
		Version:         version.Version,
		DefaultTemplate: y,
	}
	info.LimaHome, err = dirnames.LimaDir()
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
