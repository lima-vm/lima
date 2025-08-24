// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/lima-vm/lima/v2/cmd/limactl/editflags"
	"github.com/lima-vm/lima/v2/pkg/instance"
	"github.com/lima-vm/lima/v2/pkg/limayaml"
	networks "github.com/lima-vm/lima/v2/pkg/networks/reconcile"
	"github.com/lima-vm/lima/v2/pkg/store"
	"github.com/lima-vm/lima/v2/pkg/store/filenames"
	"github.com/lima-vm/lima/v2/pkg/yqutil"
)

func newCloneCommand() *cobra.Command {
	cloneCommand := &cobra.Command{
		Use:   "clone OLDINST NEWINST",
		Short: "Clone an instance of Lima",
		Long: `Clone an instance of Lima.

Not to be confused with 'limactl copy' ('limactl cp').
`,
		Args:              WrapArgsError(cobra.ExactArgs(2)),
		RunE:              cloneAction,
		ValidArgsFunction: cloneBashComplete,
		GroupID:           advancedCommand,
	}
	editflags.RegisterEdit(cloneCommand, "[limactl edit] ")
	return cloneCommand
}

func cloneAction(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	flags := cmd.Flags()
	tty, err := flags.GetBool("tty")
	if err != nil {
		return err
	}

	oldInstName, newInstName := args[0], args[1]
	oldInst, err := store.Inspect(ctx, oldInstName)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("instance %q not found", oldInstName)
		}
		return err
	}

	newInst, err := instance.Clone(ctx, oldInst, newInstName)
	if err != nil {
		return err
	}

	yqExprs, err := editflags.YQExpressions(flags, false)
	if err != nil {
		return err
	}
	if len(yqExprs) > 0 {
		// TODO: reduce duplicated codes across cloneAction and editAction
		yq := yqutil.Join(yqExprs)
		filePath := filepath.Join(newInst.Dir, filenames.LimaYAML)
		yContent, err := os.ReadFile(filePath)
		if err != nil {
			return err
		}
		yBytes, err := yqutil.EvaluateExpression(ctx, yq, yContent)
		if err != nil {
			return err
		}
		y, err := limayaml.LoadWithWarnings(ctx, yBytes, filePath)
		if err != nil {
			return err
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
		newInst, err = store.Inspect(ctx, newInst.Name)
		if err != nil {
			return err
		}
	}

	if !tty {
		// use "start" to start it
		return nil
	}
	startNow, err := askWhetherToStart()
	if err != nil {
		return err
	}
	if !startNow {
		return nil
	}
	err = networks.Reconcile(ctx, newInst.Name)
	if err != nil {
		return err
	}
	return instance.Start(ctx, newInst, "", false, false)
}

func cloneBashComplete(cmd *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	return bashCompleteInstanceNames(cmd)
}
