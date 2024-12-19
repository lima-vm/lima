package main

import (
	"fmt"
	"os"

	"github.com/lima-vm/lima/pkg/limatmpl"
	"github.com/spf13/cobra"
)

func newTemplateCommand() *cobra.Command {
	templateCommand := &cobra.Command{
		Use:           "template",
		Aliases:       []string{"tmpl"},
		Short:         "Lima template management",
		SilenceUsage:  true,
		SilenceErrors: true,
		GroupID:       advancedCommand,
		Hidden:        true,
	}
	templateCommand.AddCommand(
		newTemplateCopyCommand(),
		newTemplateValidateCommand(),
	)
	return templateCommand
}

func newTemplateCopyCommand() *cobra.Command {
	templateCopyCommand := &cobra.Command{
		Use:   "copy TEMPLATE DEST",
		Short: "Copy template",
		Args:  WrapArgsError(cobra.ExactArgs(2)),
		RunE:  templateCopyAction,
	}
	return templateCopyCommand
}

func templateCopyAction(cmd *cobra.Command, args []string) error {
	tmpl, err := limatmpl.Read(cmd.Context(), "", args[0])
	if err != nil {
		return err
	}
	if len(tmpl.Bytes) == 0 {
		return fmt.Errorf("don't know how to interpret %q as a template locator", args[0])
	}
	writer := cmd.OutOrStdout()
	target := args[1]
	if target != "-" {
		file, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
		if err != nil {
			return err
		}
		defer file.Close()
		writer = file
	}
	_, err = fmt.Fprint(writer, string(tmpl.Bytes))
	return err
}

func newTemplateValidateCommand() *cobra.Command {
	templateValidateCommand := &cobra.Command{
		Use:   "validate FILE.yaml|URL",
		Short: "Validate template",
		Args:  WrapArgsError(cobra.ExactArgs(1)),
		RunE:  validateAction,
	}
	templateValidateCommand.Flags().Bool("fill", false, "fill defaults")
	return templateValidateCommand
}
