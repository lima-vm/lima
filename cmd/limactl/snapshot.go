// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"errors"
	"fmt"
	"io"
	"text/tabwriter"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/lima-vm/lima/v2/pkg/driver"
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
	snapshots, err := snapshot.List(ctx, inst)
	if err != nil {
		return err
	}
	return printSnapshots(cmd.OutOrStdout(), snapshots, quiet)
}

func printSnapshots(w io.Writer, snapshots []driver.Snapshot, quiet bool) error {
	if quiet {
		for _, snapshot := range snapshots {
			if _, err := fmt.Fprintln(w, snapshot.Name); err != nil {
				return err
			}
		}
		return nil
	}

	tw := tabwriter.NewWriter(w, 4, 8, 4, ' ', 0)
	if _, err := fmt.Fprintln(tw, "ID\tTAG\tCREATED"); err != nil {
		return err
	}
	for _, snapshot := range snapshots {
		createdAt := "-"
		if snapshot.CreatedAt != nil {
			createdAt = snapshot.CreatedAt.Format(time.RFC3339)
		}
		if _, err := fmt.Fprintf(tw, "%s\t%s\t%s\n", snapshot.ID, snapshot.Name, createdAt); err != nil {
			return err
		}
	}
	return tw.Flush()
}

func snapshotBashComplete(cmd *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	return bashCompleteInstanceNames(cmd)
}
