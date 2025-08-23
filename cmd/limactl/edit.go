// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/lima-vm/lima/v2/cmd/limactl/editflags"
	"github.com/lima-vm/lima/v2/pkg/driverutil"
	"github.com/lima-vm/lima/v2/pkg/editutil"
	"github.com/lima-vm/lima/v2/pkg/instance"
	"github.com/lima-vm/lima/v2/pkg/limatype"
	"github.com/lima-vm/lima/v2/pkg/limatype/dirnames"
	"github.com/lima-vm/lima/v2/pkg/limatype/filenames"
	"github.com/lima-vm/lima/v2/pkg/limayaml"
	networks "github.com/lima-vm/lima/v2/pkg/networks/reconcile"
	"github.com/lima-vm/lima/v2/pkg/store"
	"github.com/lima-vm/lima/v2/pkg/uiutil"
	"github.com/lima-vm/lima/v2/pkg/yqutil"
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
	editflags.RegisterEdit(editCommand, "")
	return editCommand
}

func editAction(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	var arg string
	if len(args) > 0 {
		arg = args[0]
	}

	var filePath string
	var err error
	var inst *limatype.Instance

	if arg == "" {
		arg = DefaultInstanceName
	}
	if err := dirnames.ValidateInstName(arg); err == nil {
		inst, err = store.Inspect(ctx, arg)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("instance %q not found", arg)
			}
			return err
		}
		if inst.Status == limatype.StatusRunning {
			return errors.New("cannot edit a running instance")
		}
		filePath = filepath.Join(inst.Dir, filenames.LimaYAML)
	} else {
		// absolute path is required for `limayaml.Validate`
		filePath, err = filepath.Abs(arg)
		if err != nil {
			return err
		}
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
		yBytes, err = editutil.OpenEditor(ctx, yContent, hdr)
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
	y, err := limayaml.LoadWithWarnings(ctx, yBytes, filePath)
	if err != nil {
		return err
	}
	if err := driverutil.ResolveVMType(y, filePath); err != nil {
		return fmt.Errorf("failed to accept config for %q: %w", filePath, err)
	}
	if err := limayaml.Validate(y, true); err != nil {
		return saveRejectedYAML(yBytes, err)
	}

	if err := limayaml.ValidateAgainstLatestConfig(ctx, yBytes, yContent); err != nil {
		return saveRejectedYAML(yBytes, err)
	}

	if err := os.WriteFile(filePath, yBytes, 0o644); err != nil {
		return err
	}

	if inst != nil {
		logrus.Infof("Instance %q configuration edited", inst.Name)
	}

	if !tty {
		// use "start" to start it
		return nil
	}
	if inst == nil {
		// edited a limayaml file directly
		return nil
	}
	startNow, err := askWhetherToStart()
	if err != nil {
		return err
	}
	if !startNow {
		return nil
	}
	err = networks.Reconcile(ctx, inst.Name)
	if err != nil {
		return err
	}

	// store.Inspect() syncs values between inst.YAML and the store.
	// This call applies the validated template to the store.
	inst, err = store.Inspect(ctx, inst.Name)
	if err != nil {
		return err
	}
	return instance.Start(ctx, inst, "", false, false)
}

func askWhetherToStart() (bool, error) {
	message := "Do you want to start the instance now? "
	return uiutil.Confirm(message, true)
}

func editBashComplete(cmd *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	return bashCompleteInstanceNames(cmd)
}

// saveRejectedYAML writes the rejected config and returns an error.
func saveRejectedYAML(y []byte, origErr error) error {
	rejectedYAML := "lima.REJECTED.yaml"
	if writeErr := os.WriteFile(rejectedYAML, y, 0o644); writeErr != nil {
		return fmt.Errorf("the YAML is invalid, attempted to save the buffer as %q but failed: %w", rejectedYAML, errors.Join(writeErr, origErr))
	}
	// TODO: may need to support editing the rejected YAML
	return fmt.Errorf("the YAML is invalid, saved the buffer as %q: %w", rejectedYAML, origErr)
}
