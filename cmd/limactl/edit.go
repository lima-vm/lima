package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/lima-vm/lima/cmd/limactl/editflags"
	"github.com/lima-vm/lima/cmd/limactl/guessarg"
	"github.com/lima-vm/lima/pkg/editutil"
	"github.com/lima-vm/lima/pkg/limayaml"
	"github.com/lima-vm/lima/pkg/store"
	"github.com/lima-vm/lima/pkg/store/filenames"
	"github.com/lima-vm/lima/pkg/yqutil"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func newEditCommand() *cobra.Command {
	editCommand := &cobra.Command{
		Use:               "edit INSTANCE|FILE.yaml",
		Short:             "Edit an instance of Lima or a template",
		Args:              WrapArgsError(cobra.MaximumNArgs(1)),
		RunE:              editAction,
		ValidArgsFunction: editBashComplete,
		GroupID:           basicCommand,
	}
	editflags.RegisterEdit(editCommand)
	return editCommand
}

func editAction(cmd *cobra.Command, args []string) error {
	var arg string
	if len(args) > 0 {
		arg = args[0]
	}

	var filePath string
	var err error
	var inst *store.Instance
	switch {
	case guessarg.SeemsYAMLPath(arg):
		// absolute path is required for `limayaml.Validate`
		filePath, err = filepath.Abs(arg)
		if err != nil {
			return err
		}
	default:
		var instName string
		if arg != "" {
			instName = arg
		} else {
			instName = DefaultInstanceName
		}

		inst, err = store.Inspect(instName)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("instance %q not found", instName)
			}
			return err
		}

		if inst.Status == store.StatusRunning {
			return errors.New("cannot edit a running instance")
		}
		filePath = filepath.Join(inst.Dir, filenames.LimaYAML)
	}

	yContent, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}
	flags := cmd.Flags()
	tty, err := flags.GetBool("tty")
	if err != nil {
		return err
	}
	yqExprs, err := editflags.YQExpressions(flags, false)
	if err != nil {
		return err
	}
	var yBytes []byte
	if len(yqExprs) > 0 {
		yq := yqutil.Join(yqExprs)
		yBytes, err = yqutil.EvaluateExpression(yq, yContent)
		if err != nil {
			return err
		}
	} else if tty {
		var hdr string
		if inst != nil {
			hdr = fmt.Sprintf("# Please edit the following configuration for Lima instance %q\n", inst.Name)
		} else {
			hdr = fmt.Sprintf("# Please edit the following configuration %q\n", filePath)
		}
		hdr += "# and an empty file will abort the edit.\n"
		hdr += "\n"
		hdr += editutil.GenerateEditorWarningHeader()
		yBytes, err = editutil.OpenEditor(yContent, hdr)
		if err != nil {
			return err
		}
	}
	if len(yBytes) == 0 {
		logrus.Info("Aborting, as requested by saving the file with empty content")
		return nil
	}
	if bytes.Equal(yBytes, yContent) {
		logrus.Info("Aborting, no changes made to the instance")
		return nil
	}
	y, err := limayaml.LoadWithWarnings(yBytes, filePath)
	if err != nil {
		return err
	}
	if err := limayaml.Validate(y, true); err != nil {
		rejectedYAML := "lima.REJECTED.yaml"
		if writeErr := os.WriteFile(rejectedYAML, yBytes, 0o644); writeErr != nil {
			return fmt.Errorf("the YAML is invalid, attempted to save the buffer as %q but failed: %w: %w", rejectedYAML, writeErr, err)
		}
		// TODO: may need to support editing the rejected YAML
		return fmt.Errorf("the YAML is invalid, saved the buffer as %q: %w", rejectedYAML, err)
	}
	if err := os.WriteFile(filePath, yBytes, 0o644); err != nil {
		return err
	}
	if inst != nil {
		logrus.Infof("Instance %q configuration edited", inst.Name)
		tty, err := interactive(cmd)
		if tty && err == nil {
			err = askToStart(cmd, inst.Name, false)
		}
		return err
	}
	// inst is nil if edited a limayaml file directly
	return nil
}

func editBashComplete(cmd *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	return bashCompleteInstanceNames(cmd)
}
