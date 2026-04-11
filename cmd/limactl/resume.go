// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	hostagentclient "github.com/lima-vm/lima/v2/pkg/hostagent/api/client"
	"github.com/lima-vm/lima/v2/pkg/limatype"
	"github.com/lima-vm/lima/v2/pkg/limatype/filenames"
	"github.com/lima-vm/lima/v2/pkg/store"
)

func newResumeCommand() *cobra.Command {
	return &cobra.Command{
		Use:               "resume INSTANCE",
		Short:             "Resume a paused instance",
		Long:              "Resume a paused instance. If the instance is already running, this is a no-op.",
		Args:              WrapArgsError(cobra.MaximumNArgs(1)),
		RunE:              resumeAction,
		ValidArgsFunction: resumeBashComplete,
		GroupID:           basicCommand,
	}
}

func resumeAction(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	instName := DefaultInstanceName
	if len(args) > 0 {
		instName = args[0]
	}

	inst, err := store.Inspect(ctx, instName)
	if err != nil {
		return err
	}

	if inst.Status == limatype.StatusRunning {
		fmt.Fprintf(cmd.OutOrStdout(), "Instance %q is already running\n", instName)
		return nil
	}
	if inst.Status != limatype.StatusPaused {
		return fmt.Errorf("instance %q is not paused (status: %s)", instName, inst.Status)
	}

	haSock := filepath.Join(inst.Dir, filenames.HostAgentSock)
	haClient, err := hostagentclient.NewHostAgentClient(haSock)
	if err != nil {
		return fmt.Errorf("failed to connect to host agent: %w", err)
	}

	triggered, err := haClient.Resume(ctx)
	if err != nil {
		return fmt.Errorf("failed to resume instance %q: %w", instName, err)
	}
	if !triggered {
		fmt.Fprintf(cmd.OutOrStdout(), "Instance %q is already running\n", instName)
		return nil
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Resumed instance %q\n", instName)
	return nil
}

func resumeBashComplete(cmd *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	return bashCompleteInstanceNames(cmd)
}
