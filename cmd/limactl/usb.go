//go:build darwin && cgo

// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"text/tabwriter"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/lima-vm/lima/v2/pkg/limatype"
	"github.com/lima-vm/lima/v2/pkg/limatype/dirnames"
	"github.com/lima-vm/lima/v2/pkg/store"
	"github.com/lima-vm/lima/v2/pkg/usbip"
)

func newUsbCommand() *cobra.Command {
	usbCommand := &cobra.Command{
		Use:   "usb",
		Short: "Live USB passthrough management (vz only)",
		// Cobra runs only the nearest persistent hook, so call root's.
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			err := cmd.Root().PersistentPreRunE(cmd, args)
			logrus.Warn("`limactl usb` is experimental")
			return err
		},
		GroupID: advancedCommand,
	}
	usbCommand.AddCommand(
		newUsbListCommand(),
	)
	return usbCommand
}

func newUsbListCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "list [INSTANCE]",
		Aliases:           []string{"ls"},
		Short:             "List host USB devices and holders; INSTANCE adds names from its allowlist",
		Args:              WrapArgsError(cobra.MaximumNArgs(1)),
		RunE:              usbListAction,
		ValidArgsFunction: usbListComplete,
	}
	cmd.Flags().Bool("json", false, "Print one JSON object per line")
	return cmd
}

func usbListAction(cmd *cobra.Command, args []string) error {
	if len(args) > 0 {
		if _, err := inspectVZInstance(cmd.Context(), args[0]); err != nil {
			return err
		}
	}
	hosts, err := usbip.List()
	if err != nil {
		return fmt.Errorf("enumerating host USB devices: %w", err)
	}
	jsonFormat, _ := cmd.Flags().GetBool("json")
	return writeUSBList(cmd.OutOrStdout(), hosts, args, jsonFormat)
}

type usbListEntry struct {
	Busid      string `json:"busid"`
	VendorID   string `json:"vendorID"`
	ProductID  string `json:"productID"`
	Vendor     string `json:"vendor"`
	Product    string `json:"product"`
	Name       string `json:"name"`
	AttachedTo string `json:"attachedTo"`
}

func writeUSBList(out io.Writer, hosts []usbip.DeviceInfo, args []string, jsonFormat bool) error {
	w := tabwriter.NewWriter(out, 4, 8, 4, ' ', 0)
	if !jsonFormat {
		fmt.Fprintln(w, "BUSID\tVID:PID\tVENDOR\tPRODUCT\tNAME\tATTACHED TO")
	}
	enc := json.NewEncoder(out)
	for _, h := range hosts {
		holder, _ := usbip.Holder(h.BusID)
		nameFrom := holder
		if nameFrom == "" && len(args) > 0 {
			nameFrom = args[0]
		}
		ids := usbip.EntryFor(h)
		e := usbListEntry{h.BusID, ids.VendorID, ids.ProductID, h.VendorName, h.ProductName, allowlistName(nameFrom, h), holder}
		if jsonFormat {
			if err := enc.Encode(e); err != nil {
				return err
			}
			continue
		}
		fmt.Fprintf(w, "%s\t%s:%s\t%s\t%s\t%s\t%s\n",
			e.Busid, e.VendorID, e.ProductID,
			cmp.Or(e.Vendor, "-"), cmp.Or(e.Product, "-"),
			cmp.Or(e.Name, "-"), cmp.Or(e.AttachedTo, "-"))
	}
	return w.Flush()
}

func allowlistName(instName string, dev usbip.DeviceInfo) string {
	dir, err := dirnames.InstanceDir(instName)
	if err != nil {
		return ""
	}
	list, err := usbip.ReadAllowlist(dir)
	if err != nil {
		logrus.WithError(err).Debugf("usb: could not read allowlist of %q", instName)
	}
	for _, e := range list {
		if e.Name != "" && usbip.Allowed([]usbip.AllowEntry{e}, usbip.EntryFor(dev)) {
			return e.Name
		}
	}
	return ""
}

func inspectVZInstance(ctx context.Context, name string) (*limatype.Instance, error) {
	inst, err := store.Inspect(ctx, name)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("instance %#q does not exist, run `limactl create %s` to create a new instance", name, name)
		}
		return nil, err
	}
	if inst.Config == nil {
		return nil, fmt.Errorf("instance %q has an invalid config: %w", name, errors.Join(inst.Errors...))
	}
	if inst.VMType != limatype.VZ {
		return nil, fmt.Errorf("instance %q is not a vz instance; USB passthrough currently supports only vmType vz", name)
	}
	inst.Name = usbip.RealName(inst.Dir)
	return inst, nil
}

func usbListComplete(cmd *cobra.Command, args []string, _ string) ([]cobra.Completion, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return bashCompleteInstanceNames(cmd)
}
