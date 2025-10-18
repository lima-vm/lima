// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"strconv"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/lima-vm/lima/v2/pkg/memory"
	"github.com/lima-vm/lima/v2/pkg/store"
)

func newMemoryCommand() *cobra.Command {
	memoryCmd := &cobra.Command{
		Use:   "memory",
		Short: "Manage instance memory",
		PersistentPreRun: func(*cobra.Command, []string) {
			logrus.Warn("`limactl memory` is experimental")
		},
		GroupID: advancedCommand,
	}
	memoryCmd.AddCommand(newMemoryGetCommand())
	memoryCmd.AddCommand(newMemorySetCommand())

	return memoryCmd
}

func newMemoryGetCommand() *cobra.Command {
	getCmd := &cobra.Command{
		Use:               "get INSTANCE",
		Short:             "Get current memory",
		Long:              "Get the currently used total memory of an instance, in MiB",
		Args:              cobra.MinimumNArgs(1),
		RunE:              memoryGetAction,
		ValidArgsFunction: memoryBashComplete,
	}

	return getCmd
}

func memoryGetAction(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	instName := args[0]

	inst, err := store.Inspect(ctx, instName)
	if err != nil {
		return err
	}

	_ = inst
	mem := 0 // TODO: implement
	fmt.Fprintf(cmd.OutOrStdout(), "%d\n", mem>>20)
	return nil
}

func newMemorySetCommand() *cobra.Command {
	setCmd := &cobra.Command{
		Use:               "set INSTANCE memory AMOUNT",
		Short:             "Set target memory",
		Long:              "Set the target total memory of an instance, in MiB",
		Args:              cobra.MinimumNArgs(2),
		RunE:              memorySetAction,
		ValidArgsFunction: memoryBashComplete,
	}

	return setCmd
}

func memorySetAction(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	instName := args[0]
	meg, err := strconv.Atoi(args[1])
	if err != nil {
		return err
	}

	inst, err := store.Inspect(ctx, instName)
	if err != nil {
		return err
	}

	return memory.SetTarget(ctx, inst, int64(meg)<<20)
}

func memoryBashComplete(cmd *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	return bashCompleteInstanceNames(cmd)
}
