/*
Copyright The Lima Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/lima-vm/lima/pkg/limatmpl"
	"github.com/lima-vm/lima/pkg/limayaml"
	"github.com/lima-vm/lima/pkg/store/dirnames"
	"github.com/sirupsen/logrus"
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
		// The template command is still hidden because the subcommands and options are still under development
		// and subject to change at any time.
		Hidden: true,
	}
	templateCommand.AddCommand(
		newTemplateCopyCommand(),
		newTemplateValidateCommand(),
	)
	return templateCommand
}

// The validate command exists for backwards compatibility, and because the template command is still hidden.
func newValidateCommand() *cobra.Command {
	validateCommand := newTemplateValidateCommand()
	validateCommand.GroupID = advancedCommand
	return validateCommand
}

var templateCopyExample = `  Template locators are local files, file://, https://, or template:// URLs

  # Copy default template to STDOUT
  limactl template copy template://default -

  # Copy template from web location to local file
  limactl template copy https://example.com/lima.yaml mighty-machine.yaml
`

func newTemplateCopyCommand() *cobra.Command {
	templateCopyCommand := &cobra.Command{
		Use:     "copy TEMPLATE DEST",
		Short:   "Copy template",
		Long:    "Copy a template via locator to a local file",
		Example: templateCopyExample,
		Args:    WrapArgsError(cobra.ExactArgs(2)),
		RunE:    templateCopyAction,
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
		Use:   "validate TEMPLATE [TEMPLATE, ...]",
		Short: "Validate YAML templates",
		Args:  WrapArgsError(cobra.MinimumNArgs(1)),
		RunE:  templateValidateAction,
	}
	templateValidateCommand.Flags().Bool("fill", false, "fill defaults")
	return templateValidateCommand
}

func templateValidateAction(cmd *cobra.Command, args []string) error {
	fill, err := cmd.Flags().GetBool("fill")
	if err != nil {
		return err
	}
	limaDir, err := dirnames.LimaDir()
	if err != nil {
		return err
	}

	for _, arg := range args {
		tmpl, err := limatmpl.Read(cmd.Context(), "", arg)
		if err != nil {
			return err
		}
		if len(tmpl.Bytes) == 0 {
			return fmt.Errorf("don't know how to interpret %q as a template locator", arg)
		}
		if tmpl.Name == "" {
			return fmt.Errorf("can't determine instance name from template locator %q", arg)
		}
		// Load() will merge the template with override.yaml and default.yaml via FillDefaults().
		// FillDefaults() needs the potential instance directory to validate host templates using {{.Dir}}.
		instDir := filepath.Join(limaDir, tmpl.Name)
		y, err := limayaml.Load(tmpl.Bytes, instDir)
		if err != nil {
			return err
		}
		if err := limayaml.Validate(y, false); err != nil {
			return fmt.Errorf("failed to validate YAML file %q: %w", arg, err)
		}
		logrus.Infof("%q: OK", arg)
		if fill {
			b, err := limayaml.Marshal(y, len(args) > 1)
			if err != nil {
				return fmt.Errorf("failed to marshal template %q again after filling defaults: %w", arg, err)
			}
			fmt.Fprint(cmd.OutOrStdout(), string(b))
		}
	}

	return nil
}
