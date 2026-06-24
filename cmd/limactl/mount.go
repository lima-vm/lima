// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"
	"path/filepath"
	"text/tabwriter"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/lima-vm/lima/v2/pkg/hostagent/api"
	hostagentclient "github.com/lima-vm/lima/v2/pkg/hostagent/api/client"
	"github.com/lima-vm/lima/v2/pkg/limatype"
	"github.com/lima-vm/lima/v2/pkg/limatype/filenames"
	"github.com/lima-vm/lima/v2/pkg/store"
)

func newMountCommand() *cobra.Command {
	mountCommand := &cobra.Command{
		Use:     "mount",
		Short:   "Mount and unmount host directories in a running instance at runtime",
		GroupID: advancedCommand,
	}
	mountCommand.AddCommand(
		newMountAddCommand(),
		newMountRemoveCommand(),
		newMountListCommand(),
	)
	return mountCommand
}

func newMountAddCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add INSTANCE HOST_PATH GUEST_PATH",
		Short: "Mount a host directory into a running instance (QEMU/Linux)",
		Long: `Mount a host directory into a running instance without restarting it.

The default transport is virtiofs (high throughput). 9p and reverse-sshfs are also
available via --type. Runtime mounts are ephemeral and are not written to lima.yaml.`,
		Args:              WrapArgsError(cobra.ExactArgs(3)),
		RunE:              mountAddAction,
		ValidArgsFunction: mountBashComplete,
	}
	cmd.Flags().String("type", "", "mount type: virtiofs (default), 9p, or reverse-sshfs")
	cmd.Flags().Bool("writable", false, "mount as writable")
	return cmd
}

func mountAddAction(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	instName, hostPath, guestPath := args[0], args[1], args[2]
	mountType, err := cmd.Flags().GetString("type")
	if err != nil {
		return err
	}
	if err := validateMountType(mountType); err != nil {
		return err
	}
	writable, err := cmd.Flags().GetBool("writable")
	if err != nil {
		return err
	}
	absHostPath, err := filepath.Abs(hostPath)
	if err != nil {
		return err
	}
	client, err := mountHostAgentClient(ctx, instName)
	if err != nil {
		return err
	}
	m, err := client.MountAdd(ctx, &api.MountRequest{
		HostPath:   absHostPath,
		MountPoint: guestPath,
		Type:       mountType,
		Writable:   writable,
	})
	if err != nil {
		return err
	}
	logrus.Infof("Mounted %#q on %#q (%s)", m.HostPath, m.MountPoint, m.Type)
	return nil
}

func newMountRemoveCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "remove INSTANCE GUEST_PATH",
		Aliases:           []string{"rm", "unmount"},
		Short:             "Unmount a runtime mount from a running instance",
		Args:              WrapArgsError(cobra.ExactArgs(2)),
		RunE:              mountRemoveAction,
		ValidArgsFunction: mountBashComplete,
	}
	return cmd
}

func mountRemoveAction(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	client, err := mountHostAgentClient(ctx, args[0])
	if err != nil {
		return err
	}
	if err := client.MountRemove(ctx, args[1]); err != nil {
		return err
	}
	logrus.Infof("Unmounted %#q", args[1])
	return nil
}

func newMountListCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "list INSTANCE",
		Aliases:           []string{"ls"},
		Short:             "List the runtime mounts of a running instance",
		Args:              WrapArgsError(cobra.ExactArgs(1)),
		RunE:              mountListAction,
		ValidArgsFunction: mountBashComplete,
	}
	return cmd
}

func mountListAction(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	client, err := mountHostAgentClient(ctx, args[0])
	if err != nil {
		return err
	}
	mounts, err := client.MountList(ctx)
	if err != nil {
		return err
	}
	w := tabwriter.NewWriter(cmd.OutOrStdout(), 4, 8, 4, ' ', 0)
	fmt.Fprintln(w, "MOUNTPOINT\tTYPE\tWRITABLE\tHOSTPATH")
	for _, m := range mounts {
		fmt.Fprintf(w, "%s\t%s\t%v\t%s\n", m.MountPoint, m.Type, m.Writable, m.HostPath)
	}
	return w.Flush()
}

func validateMountType(mountType string) error {
	switch mountType {
	case "", "virtiofs", "9p", "reverse-sshfs":
		return nil
	default:
		return fmt.Errorf("invalid --type %#q (must be virtiofs, 9p, or reverse-sshfs)", mountType)
	}
}

// mountHostAgentClient resolves a running instance and connects to its host agent socket.
func mountHostAgentClient(ctx context.Context, instName string) (hostagentclient.HostAgentClient, error) {
	inst, err := store.Inspect(ctx, instName)
	if err != nil {
		return nil, fmt.Errorf("instance %#q not found: %w", instName, err)
	}
	if inst.Status != limatype.StatusRunning {
		return nil, fmt.Errorf("instance %#q is not running (status: %s)", instName, inst.Status)
	}
	haSock := filepath.Join(inst.Dir, filenames.HostAgentSock)
	return hostagentclient.NewHostAgentClient(haSock)
}

func mountBashComplete(cmd *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	return bashCompleteInstanceNames(cmd)
}
