package main

import (
	"fmt"

	"github.com/lima-vm/lima/pkg/limatmpl"
	"github.com/spf13/cobra"
)

func newTemplateCommand() *cobra.Command {
	validateCommand := &cobra.Command{
		Use:     "template FILE.yaml|URL",
		Short:   "Assemble template and print it",
		Args:    WrapArgsError(cobra.ExactArgs(1)),
		RunE:    templateAction,
		GroupID: advancedCommand,
	}
	return validateCommand
}

func templateAction(cmd *cobra.Command, args []string) error {
	tmpl, err := limatmpl.Read(cmd.Context(), "", args[0])
	if err != nil {
		return err
	}
	if len(tmpl.Bytes) == 0 {
		return fmt.Errorf("don't know how to interpret %q as a template locator", args[0])
	}
	_, err = fmt.Fprint(cmd.OutOrStdout(), string(tmpl.Bytes))
	return err
}
