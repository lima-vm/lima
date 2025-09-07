// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"fmt"

	"github.com/mikefarah/yq/v4/pkg/yqlib"
	"github.com/spf13/cobra"

	"github.com/lima-vm/lima/v2/pkg/limainfo"
	"github.com/lima-vm/lima/v2/pkg/uiutil"
	"github.com/lima-vm/lima/v2/pkg/yqutil"
)

func newInfoCommand() *cobra.Command {
	infoCommand := &cobra.Command{
		Use:     "info",
		Short:   "Show diagnostic information",
		Args:    WrapArgsError(cobra.NoArgs),
		RunE:    infoAction,
		GroupID: advancedCommand,
	}
	infoCommand.Flags().String("yq", ".", "Apply yq expression to output")

	return infoCommand
}

func infoAction(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()

	yq, err := cmd.Flags().GetString("yq")
	if err != nil {
		return err
	}

	info, err := limainfo.New(ctx)
	if err != nil {
		return err
	}
	j, err := json.MarshalIndent(info, "", "    ")
	if err != nil {
		return err
	}

	encoderPrefs := yqlib.ConfiguredJSONPreferences.Copy()
	encoderPrefs.Indent = 4
	encoderPrefs.ColorsEnabled = uiutil.OutputIsTTY(cmd.OutOrStdout())
	encoder := yqlib.NewJSONEncoder(encoderPrefs)
	str, err := yqutil.EvaluateExpressionWithEncoder(yq, string(j), encoder)
	if err == nil {
		_, err = fmt.Fprint(cmd.OutOrStdout(), str)
	}
	return err
}
