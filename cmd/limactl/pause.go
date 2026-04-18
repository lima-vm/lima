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

func newPauseCommand() *cobra.Command {
	return &cobra.Command{
		Use:               "pause INSTANCE",
		Short:             "Pause a running instance",
		Long:              "Pause a running instance immediately. Requires auto-pause to be enabled in the instance configuration. The instance can be resumed with 'limactl resume' or automatically when a client connects to a forwarded socket.",
		Args:              WrapArgsError(cobra.MaximumNArgs(1)),
		RunE:              pauseAction,
		ValidArgsFunction: pauseBashComplete,
		GroupID:           basicCommand,
	}
}

func pauseAction(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	instName := DefaultInstanceName
	if len(args) > 0 {
		instName = args[0]
	}

	inst, err := store.Inspect(ctx, instName)
	if err != nil {
		return err
	}

	if inst.Status == limatype.StatusPaused {
		return fmt.Errorf("instance %q is already paused", instName)
	}
	if inst.Status != limatype.StatusRunning {
		return fmt.Errorf("instance %q is not running (status: %s)", instName, inst.Status)
	}

	haSock := filepath.Join(inst.Dir, filenames.HostAgentSock)
	haClient, err := hostagentclient.NewHostAgentClient(haSock)
	if err != nil {
		return fmt.Errorf("failed to connect to host agent: %w", err)
	}

	if err := haClient.Pause(ctx); err != nil {
		return fmt.Errorf("failed to pause instance %q: %w", instName, err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Paused instance %q\n", instName)
	return nil
}

func pauseBashComplete(cmd *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	return bashCompleteInstanceNames(cmd)
}
