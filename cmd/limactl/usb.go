// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/lima-vm/lima/v2/pkg/driver"
	"github.com/lima-vm/lima/v2/pkg/hostagent/api/client"
	"github.com/lima-vm/lima/v2/pkg/limatype"
	"github.com/lima-vm/lima/v2/pkg/limatype/filenames"
	"github.com/lima-vm/lima/v2/pkg/store"
)

func newUSBCommand() *cobra.Command {
	usbCommand := &cobra.Command{
		Use:   "usb",
		Short: "Attach host USB devices to an instance (vz, macOS 27+)",
		Long: `Attach host USB devices to a running instance.

USB passthrough needs the vz driver on macOS 27 or later, built with the macOS 27 SDK
and signed with the com.apple.developer.accessory-access.usb entitlement,
and the instance to have vmOpts.vz.usb.enabled set to true, or vmOpts.vz.usb.devices set.
See https://lima-vm.io/docs/config/usb/ .

Devices attached with this command are not saved to the instance configuration;
to attach devices automatically, set vmOpts.vz.usb.devices.`,
		Example: `  List the host USB devices:
  $ limactl usb list INSTANCE

  Attach a device by vendor and product ID, or by registry ID:
  $ limactl usb attach INSTANCE 05ac:12a8
  $ limactl usb attach INSTANCE 0x100000abc

  Detach a device:
  $ limactl usb detach INSTANCE 05ac:12a8`,
		SilenceUsage:  true,
		SilenceErrors: true,
		GroupID:       advancedCommand,
	}
	usbCommand.AddCommand(
		newUSBListCommand(),
		newUSBAttachCommand(),
		newUSBDetachCommand(),
	)
	return usbCommand
}

func newUSBListCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "list INSTANCE",
		Aliases:           []string{"ls"},
		Short:             "List the host USB devices that can be attached",
		Args:              WrapArgsError(cobra.ExactArgs(1)),
		RunE:              usbListAction,
		ValidArgsFunction: usbBashComplete,
	}
	cmd.Flags().Bool("json", false, "JSONify output")
	return cmd
}

func newUSBAttachCommand() *cobra.Command {
	return &cobra.Command{
		Use:               "attach INSTANCE DEVICE",
		Short:             "Attach a host USB device, specified as VENDOR:PRODUCT (hex) or registry ID",
		Args:              WrapArgsError(cobra.ExactArgs(2)),
		RunE:              usbAttachAction,
		ValidArgsFunction: usbBashComplete,
	}
}

func newUSBDetachCommand() *cobra.Command {
	return &cobra.Command{
		Use:               "detach INSTANCE DEVICE",
		Short:             "Detach a host USB device, specified as VENDOR:PRODUCT (hex) or registry ID",
		Args:              WrapArgsError(cobra.ExactArgs(2)),
		RunE:              usbDetachAction,
		ValidArgsFunction: usbBashComplete,
	}
}

// macOS gives Lima only the devices assigned to it.
const usbAssignHint = "Check that the device is connected, and assigned to the app that started the instance " +
	"(e.g., Terminal) under \"Use with\" in the \"Virtual Machine Accessories\" menu bar item"

func usbHostAgentClient(ctx context.Context, instName string) (client.HostAgentClient, error) {
	inst, err := store.Inspect(ctx, instName)
	if err != nil {
		return nil, err
	}
	if inst.Status != limatype.StatusRunning {
		return nil, fmt.Errorf("instance %q is not running (status: %s)", instName, inst.Status)
	}
	haClient, err := client.NewHostAgentClient(filepath.Join(inst.Dir, filenames.HostAgentSock))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to hostagent: %w", err)
	}
	return haClient, nil
}

func usbListAction(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	haClient, err := usbHostAgentClient(ctx, args[0])
	if err != nil {
		return err
	}
	jsonFormat, err := cmd.Flags().GetBool("json")
	if err != nil {
		return err
	}
	devs, err := haClient.ListUSBDevices(ctx)
	if err != nil {
		return err
	}
	if jsonFormat {
		for _, dev := range devs {
			j, err := json.Marshal(dev)
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), string(j))
		}
		return nil
	}
	if len(devs) == 0 {
		logrus.Warn("No USB device found. " + usbAssignHint)
	}
	w := tabwriter.NewWriter(cmd.OutOrStdout(), 4, 8, 4, ' ', 0)
	fmt.Fprintln(w, "ID\tVENDOR:PRODUCT\tATTACHED\tMANUFACTURER\tPRODUCT\tSERIAL")
	for _, dev := range devs {
		fmt.Fprintf(w, "%#x\t%04x:%04x\t%t\t%s\t%s\t%s\n",
			dev.RegistryID, dev.VendorID, dev.ProductID, dev.Attached, dev.Manufacturer, dev.Product, dev.Serial)
	}
	return w.Flush()
}

func usbAttachAction(cmd *cobra.Command, args []string) error {
	return usbAttachDetach(cmd.Context(), args[0], args[1], true)
}

func usbDetachAction(cmd *cobra.Command, args []string) error {
	return usbAttachDetach(cmd.Context(), args[0], args[1], false)
}

func usbAttachDetach(ctx context.Context, instName, device string, attach bool) error {
	haClient, err := usbHostAgentClient(ctx, instName)
	if err != nil {
		return err
	}
	devs, err := haClient.ListUSBDevices(ctx)
	if err != nil {
		return err
	}
	dev, err := resolveUSBDevice(devs, device, attach)
	if err != nil {
		return err
	}
	if attach {
		if err := haClient.AttachUSBDevice(ctx, dev.RegistryID); err != nil {
			return err
		}
		logrus.Infof("Attached USB device %04x:%04x (%#x) to %q", dev.VendorID, dev.ProductID, dev.RegistryID, instName)
		return nil
	}
	if err := haClient.DetachUSBDevice(ctx, dev.RegistryID); err != nil {
		return err
	}
	logrus.Infof("Detached USB device %04x:%04x (%#x) from %q", dev.VendorID, dev.ProductID, dev.RegistryID, instName)
	return nil
}

// resolveUSBDevice accepts "VENDOR:PRODUCT" in hex or a registry ID. Of several
// VENDOR:PRODUCT matches, only those in the opposite state are considered.
func resolveUSBDevice(devs []driver.USBDevice, spec string, attach bool) (driver.USBDevice, error) {
	if vendor, product, ok := strings.Cut(spec, ":"); ok {
		vendorID, err := strconv.ParseUint(strings.TrimPrefix(vendor, "0x"), 16, 16)
		if err != nil {
			return driver.USBDevice{}, fmt.Errorf("invalid vendor ID %q: %w", vendor, err)
		}
		productID, err := strconv.ParseUint(strings.TrimPrefix(product, "0x"), 16, 16)
		if err != nil {
			return driver.USBDevice{}, fmt.Errorf("invalid product ID %q: %w", product, err)
		}
		var matches []driver.USBDevice
		for _, dev := range devs {
			if uint64(dev.VendorID) == vendorID && uint64(dev.ProductID) == productID {
				matches = append(matches, dev)
			}
		}
		if len(matches) > 1 {
			inState := slices.DeleteFunc(slices.Clone(matches), func(dev driver.USBDevice) bool { return dev.Attached == attach })
			if len(inState) > 0 {
				matches = inState
			}
		}
		switch len(matches) {
		case 0:
			return driver.USBDevice{}, fmt.Errorf("no USB device %q found. %s", spec, usbAssignHint)
		case 1:
			return matches[0], nil
		default:
			return driver.USBDevice{}, fmt.Errorf("%d USB devices match %q; specify the registry ID from `limactl usb list`", len(matches), spec)
		}
	}
	registryID, err := strconv.ParseUint(spec, 0, 64)
	if err != nil {
		return driver.USBDevice{}, errors.New("device must be VENDOR:PRODUCT in hex, or a registry ID")
	}
	for _, dev := range devs {
		if dev.RegistryID == registryID {
			return dev, nil
		}
	}
	return driver.USBDevice{}, fmt.Errorf("no USB device with registry ID %#x found. %s", registryID, usbAssignHint)
}

func usbBashComplete(cmd *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
	if len(args) == 0 {
		return bashCompleteInstanceNames(cmd)
	}
	return nil, cobra.ShellCompDirectiveNoFileComp
}
