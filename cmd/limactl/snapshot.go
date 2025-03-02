/*
Copyright The Lima Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"errors"
	"fmt"
	"strings"

	"github.com/lima-vm/lima/pkg/snapshot"
	"github.com/lima-vm/lima/pkg/store"
	"github.com/sirupsen/logrus"

	"github.com/spf13/cobra"
)

func newSnapshotCommand() *cobra.Command {
	snapshotCmd := &cobra.Command{
		Use:   "snapshot",
		Short: "Manage instance snapshots",
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
		Use:               "create INSTANCE",
		Aliases:           []string{"save"},
		Short:             "Create (save) a snapshot",
		Args:              cobra.MinimumNArgs(1),
		RunE:              snapshotCreateAction,
		ValidArgsFunction: snapshotBashComplete,
	}
	createCmd.Flags().String("tag", "", "name of the snapshot")

	return createCmd
}

func snapshotCreateAction(cmd *cobra.Command, args []string) error {
	instName := args[0]

	inst, err := store.Inspect(instName)
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

	ctx := cmd.Context()
	return snapshot.Save(ctx, inst, tag)
}

func newSnapshotDeleteCommand() *cobra.Command {
	deleteCmd := &cobra.Command{
		Use:               "delete INSTANCE",
		Aliases:           []string{"del"},
		Short:             "Delete (del) a snapshot",
		Args:              cobra.MinimumNArgs(1),
		RunE:              snapshotDeleteAction,
		ValidArgsFunction: snapshotBashComplete,
	}
	deleteCmd.Flags().String("tag", "", "name of the snapshot")

	return deleteCmd
}

func snapshotDeleteAction(cmd *cobra.Command, args []string) error {
	instName := args[0]

	inst, err := store.Inspect(instName)
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

	ctx := cmd.Context()
	return snapshot.Del(ctx, inst, tag)
}

func newSnapshotApplyCommand() *cobra.Command {
	applyCmd := &cobra.Command{
		Use:               "apply INSTANCE",
		Aliases:           []string{"load"},
		Short:             "Apply (load) a snapshot",
		Args:              cobra.MinimumNArgs(1),
		RunE:              snapshotApplyAction,
		ValidArgsFunction: snapshotBashComplete,
	}
	applyCmd.Flags().String("tag", "", "name of the snapshot")

	return applyCmd
}

func snapshotApplyAction(cmd *cobra.Command, args []string) error {
	instName := args[0]

	inst, err := store.Inspect(instName)
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

	ctx := cmd.Context()
	return snapshot.Load(ctx, inst, tag)
}

func newSnapshotListCommand() *cobra.Command {
	listCmd := &cobra.Command{
		Use:               "list INSTANCE",
		Aliases:           []string{"ls"},
		Short:             "List existing snapshots",
		Args:              cobra.MinimumNArgs(1),
		RunE:              snapshotListAction,
		ValidArgsFunction: snapshotBashComplete,
	}
	listCmd.Flags().BoolP("quiet", "q", false, "Only show tags")

	return listCmd
}

func snapshotListAction(cmd *cobra.Command, args []string) error {
	instName := args[0]

	inst, err := store.Inspect(instName)
	if err != nil {
		return err
	}

	quiet, err := cmd.Flags().GetBool("quiet")
	if err != nil {
		return err
	}
	ctx := cmd.Context()
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
