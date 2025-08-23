// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/lima-vm/lima/v2/pkg/limainfo"
)

func newInfoCommand() *cobra.Command {
	infoCommand := &cobra.Command{
		Use:     "info",
		Short:   "Show diagnostic information",
		Args:    WrapArgsError(cobra.NoArgs),
		RunE:    infoAction,
		GroupID: advancedCommand,
	}
	return infoCommand
}

func infoAction(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()
	info, err := limainfo.New(ctx)
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
