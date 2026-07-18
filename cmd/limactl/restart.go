// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/lima-vm/lima/v2/cmd/limactl/editflags"
	"github.com/lima-vm/lima/v2/pkg/driverutil"
	"github.com/lima-vm/lima/v2/pkg/instance"
	"github.com/lima-vm/lima/v2/pkg/limatype"
	"github.com/lima-vm/lima/v2/pkg/limatype/filenames"
	"github.com/lima-vm/lima/v2/pkg/limayaml"
	"github.com/lima-vm/lima/v2/pkg/store"
	"github.com/lima-vm/lima/v2/pkg/yqutil"
)

func newRestartCommand() *cobra.Command {
	restartCmd := &cobra.Command{
		Use:               "restart INSTANCE",
		Short:             "Restart a running instance",
		Args:              WrapArgsError(cobra.MaximumNArgs(1)),
		RunE:              restartAction,
		ValidArgsFunction: restartBashComplete,
		GroupID:           basicCommand,
	}

	restartCmd.Flags().BoolP("force", "f", false, "Force stop and restart the instance")
	restartCmd.Flags().Bool("progress", false, "Show provision script progress by tailing cloud-init logs")
	editflags.RegisterEdit(restartCmd, "")
	return restartCmd
}

func restartAction(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	instName := DefaultInstanceName
	if len(args) > 0 {
		instName = args[0]
	}

	inst, err := store.Inspect(ctx, instName)
	if err != nil {
		return err
	}

	flags := cmd.Flags()
	force, err := flags.GetBool("force")
	if err != nil {
		return err
	}
	progress, err := flags.GetBool("progress")
	if err != nil {
		return err
	}

	filePath := filepath.Join(inst.Dir, filenames.LimaYAML)
	yContent, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	var params map[string]string
	if flags.Changed("param") {
		var y limatype.LimaYAML
		if err := limayaml.Unmarshal(yContent, &y, filePath); err != nil {
			return err
		}
		params = y.Param
	}

	yqExprs, err := editflags.YQExpressions(flags, false, params)
	if err != nil {
		return err
	}

	if len(yqExprs) > 0 {
		yq := yqutil.Join(yqExprs)
		yBytes, err := yqutil.EvaluateExpression(ctx, yq, yContent)
		if err != nil {
			return err
		}
		if !bytes.Equal(yBytes, yContent) {
			y, err := limayaml.LoadWithWarnings(ctx, yBytes, filePath)
			if err != nil {
				return err
			}
			if err := driverutil.ResolveVMType(ctx, y, filePath); err != nil {
				return fmt.Errorf("failed to resolve vm for %#q: %w", filePath, err)
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
			logrus.Infof("Instance %#q configuration edited", inst.Name)
			inst, err = store.Inspect(ctx, instName)
			if err != nil {
				return err
			}
		}
	}

	if force {
		return instance.RestartForcibly(ctx, inst, progress)
	}
	return instance.Restart(ctx, inst, progress)
}

func restartBashComplete(cmd *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	return bashCompleteInstanceNames(cmd)
}
