// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"errors"

	"github.com/spf13/cobra"

	"github.com/lima-vm/lima/v2/pkg/usbip"
)

func newUsbipCommand() *cobra.Command {
	usbipCommand := &cobra.Command{
		Use:   "usbip",
		Short: "Import host USB devices over vsock onto vhci-hcd",
	}
	usbipCommand.AddCommand(newUsbipAttachCommand(), newUsbipProbeCommand())
	return usbipCommand
}

func newUsbipProbeCommand() *cobra.Command {
	return &cobra.Command{
		Use:           "probe",
		Short:         "Exit non-zero unless the host serves USB/IP",
		Args:          cobra.NoArgs,
		RunE:          func(*cobra.Command, []string) error { return usbip.Probe() },
		SilenceUsage:  true,
		SilenceErrors: true,
	}
}

func newUsbipAttachCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "attach",
		Short: "Import a host USB device (--busid) or all advertised devices (--all)",
		Args:  cobra.NoArgs,
		RunE:  usbipAttachAction,
		// Cobra reads these on the executed command or the root, not the parent.
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	cmd.Flags().String("busid", "", "host busid to import")
	cmd.Flags().Bool("all", false, "import every device the host advertises")
	return cmd
}

func usbipAttachAction(cmd *cobra.Command, _ []string) error {
	all, _ := cmd.Flags().GetBool("all")
	busid, _ := cmd.Flags().GetString("busid")
	switch {
	case all:
		return usbip.AttachAll()
	case busid != "":
		return usbip.AttachBusid(busid)
	default:
		return errors.New("specify --busid or --all")
	}
}
