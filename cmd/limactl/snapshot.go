// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"errors"
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/lima-vm/lima/v2/pkg/snapshot"
	"github.com/lima-vm/lima/v2/pkg/store"
)

func newSnapshotCommand() *cobra.Command {
	snapshotCmd := &cobra.Command{
		Use:   "snapshot",
		Short: "Manage instance snapshots",
		Example: `  List all snapshots of an instance:
  $ limactl snapshot list default

  Create a snapshot:
  $ limactl snapshot create default --tag snap1

  Apply (restore) a snapshot:
  $ limactl snapshot apply default --tag snap1

  Delete a snapshot:
  $ limactl snapshot delete default --tag snap1
`,
		PersistentPreRun: func(*cobra.Command, []string) {
			logrus.Warn("`limactl snapshot` is experimental")
		},
		GroupID: advancedCommand,
	}
	snapshotCmd.AddCommand(newSnapshotApplyCommand())
	snapshotCmd.AddCommand(newSnapshotCreateCommand())
	snapshotCmd.AddCommand(newSnapshotDeleteCommand())
	snapshotCmd.AddCommand(newSnapshotListCommand())

	return snapshotCmd
}

func newSnapshotCreateCommand() *cobra.Command {
	createCmd := &cobra.Command{
		Use:     "create INSTANCE",
		Aliases: []string{"save"},
		Short:   "Create (save) a snapshot",
		Example: `  Create a snapshot of an instance:
  $ limactl snapshot create default --tag snap1
`,
		Args:              cobra.MinimumNArgs(1),
		RunE:              snapshotCreateAction,
		ValidArgsFunction: snapshotBashComplete,
	}
	createCmd.Flags().String("tag", "", "Name of the snapshot")

	return createCmd
}

func snapshotCreateAction(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	instName := args[0]

	inst, err := store.Inspect(ctx, instName)
	if err != nil {
		return err
	}

	tag, err := cmd.Flags().GetString("tag")
	if err != nil {
		return err
	}

	if tag == "" {
		return errors.New("expected tag")
	}

	return snapshot.Save(ctx, inst, tag)
}

func newSnapshotDeleteCommand() *cobra.Command {
	deleteCmd := &cobra.Command{
		Use:     "delete INSTANCE",
		Aliases: []string{"del"},
		Short:   "Delete (del) a snapshot",
		Example: `  Delete a snapshot:
  $ limactl snapshot delete default --tag snap1
`,
		Args:              cobra.MinimumNArgs(1),
		RunE:              snapshotDeleteAction,
		ValidArgsFunction: snapshotBashComplete,
	}
	deleteCmd.Flags().String("tag", "", "Name of the snapshot")

	return deleteCmd
}

func snapshotDeleteAction(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	instName := args[0]

	inst, err := store.Inspect(ctx, instName)
	if err != nil {
		return err
	}

	tag, err := cmd.Flags().GetString("tag")
	if err != nil {
		return err
	}

	if tag == "" {
		return errors.New("expected tag")
	}

	return snapshot.Del(ctx, inst, tag)
}

func newSnapshotApplyCommand() *cobra.Command {
	applyCmd := &cobra.Command{
		Use:     "apply INSTANCE",
		Aliases: []string{"load"},
		Short:   "Apply (load) a snapshot",
		Example: `  Apply (restore) a snapshot:
  $ limactl snapshot apply default --tag snap1
`,
		Args:              cobra.MinimumNArgs(1),
		RunE:              snapshotApplyAction,
		ValidArgsFunction: snapshotBashComplete,
	}
	applyCmd.Flags().String("tag", "", "Name of the snapshot")

	return applyCmd
}

func snapshotApplyAction(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	instName := args[0]

	inst, err := store.Inspect(ctx, instName)
	if err != nil {
		return err
	}

	tag, err := cmd.Flags().GetString("tag")
	if err != nil {
		return err
	}

	if tag == "" {
		return errors.New("expected tag")
	}

	return snapshot.Load(ctx, inst, tag)
}

func newSnapshotListCommand() *cobra.Command {
	listCmd := &cobra.Command{
		Use:     "list INSTANCE",
		Aliases: []string{"ls"},
		Short:   "List existing snapshots",
		Example: `  List all snapshots of an instance:
  $ limactl snapshot list default

  List only snapshot tags:
  $ limactl snapshot list default --quiet
`,
		Args:              cobra.MinimumNArgs(1),
		RunE:              snapshotListAction,
		ValidArgsFunction: snapshotBashComplete,
	}
	listCmd.Flags().BoolP("quiet", "q", false, "Only show tags")

	return listCmd
}

func snapshotListAction(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	instName := args[0]

	inst, err := store.Inspect(ctx, instName)
	if err != nil {
		return err
	}

	quiet, err := cmd.Flags().GetBool("quiet")
	if err != nil {
		return err
	}
	out, err := snapshot.List(ctx, inst)
	if err != nil {
		return err
	}
	if quiet {
		for i, line := range strings.Split(out, "\n") {
			// "ID", "TAG", "VM SIZE", "DATE", "VM CLOCK", "ICOUNT"
			fields := strings.Fields(line)
			if i == 0 && len(fields) > 1 && fields[1] != "TAG" {
				// make sure that output matches the expected
				return fmt.Errorf("unknown header: %s", line)
			}
			if i == 0 || line == "" {
				// skip header and empty line after using split
				continue
			}
			tag := fields[1]
			fmt.Fprintf(cmd.OutOrStdout(), "%s\n", tag)
		}
		return nil
	}
	fmt.Fprint(cmd.OutOrStdout(), out)
	return nil
}

func snapshotBashComplete(cmd *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	return bashCompleteInstanceNames(cmd)
}
