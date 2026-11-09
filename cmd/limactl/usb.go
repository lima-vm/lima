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
	"os/exec"
	"slices"
	"strconv"
	"text/tabwriter"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/lima-vm/lima/v2/pkg/limatype"
	"github.com/lima-vm/lima/v2/pkg/limatype/dirnames"
	"github.com/lima-vm/lima/v2/pkg/sshutil"
	"github.com/lima-vm/lima/v2/pkg/store"
	"github.com/lima-vm/lima/v2/pkg/strutil"
	"github.com/lima-vm/lima/v2/pkg/usbip"
)

var (
	guestCommand = runGuestCommand
	// Covers the import plus the guest's 15 s enumeration wait.
	guestTimeout = time.Minute
)

const guestOutputLimit = 64 << 10

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
		newUsbAttachCommand(),
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

func newUsbAttachCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "attach INSTANCE [BUSID]",
		Short: "Attach a host USB device to a running vz instance",
		Long: `Attach a host USB device to a running vz instance.

The attachment ends at unplug, "limactl usb detach" or VM stop. A device that
is replugged or re-enumerates stays detached until attached again, or until the
next guest boot if usb.devices matches it.`,
		Args:              WrapArgsError(cobra.RangeArgs(1, 2)),
		RunE:              usbAttachAction,
		ValidArgsFunction: usbBusidComplete,
	}
	cmd.Flags().String("vendor", "", "vendor ID in hex, e.g. 0bda")
	cmd.Flags().String("product", "", "product ID in hex, e.g. 8812")
	cmd.MarkFlagsRequiredTogether("vendor", "product")
	cmd.Flags().String("name", "", "friendly name for a new grant, ignored if the device is already allowed")
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

func usbTarget(cmd *cobra.Command, args []string) (busid, vendor, product string, err error) {
	vendor, _ = cmd.Flags().GetString("vendor")
	product, _ = cmd.Flags().GetString("product")
	if len(args) < 2 {
		return "", vendor, product, nil
	}
	if vendor+product != "" {
		return "", "", "", errors.New("specify a BUSID or --vendor and --product, not both")
	}
	return args[1], "", "", nil
}

func usbAttachAction(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	busid, vendor, product, err := usbTarget(cmd, args)
	if err != nil {
		return err
	}
	inst, err := requireVZUSBInstance(ctx, args[0])
	if err != nil {
		return err
	}
	name, _ := cmd.Flags().GetString("name")

	dev, err := resolveHostDevice(busid, vendor, product)
	if err != nil {
		return err
	}
	return attachDevice(ctx, inst, dev, name)
}

func attachDevice(ctx context.Context, inst *limatype.Instance, dev usbip.DeviceInfo, name string) error {
	if holder, held := usbip.Holder(dev.BusID); held {
		if holder == inst.Name {
			return fmt.Errorf("device %s (%04x:%04x) is already attached to instance %q", dev.BusID, dev.Vendor, dev.Product, holder)
		}
		return fmt.Errorf("device %s (%04x:%04x) is already attached to instance %q; detach it there first", dev.BusID, dev.Vendor, dev.Product, holder)
	}

	grant := usbip.EntryFor(dev)
	grant.Name = name
	added := false
	var before []usbip.AllowEntry
	err := usbip.UpdateAllowlist(inst.Dir, func(list []usbip.AllowEntry) []usbip.AllowEntry {
		// A YAML entry already allows it and must outlive this attach.
		// --name is ignored for an allowed device; merge names if that ever matters.
		added = !usbip.Allowed(list, grant)
		if !added {
			return list
		}
		before = slices.Clone(list)
		return usbip.AddEntry(list, grant)
	})
	if err != nil {
		return err
	}

	out, err := guestCommand(ctx, inst, fmt.Sprintf("sudo %s usbip attach --busid %s", *inst.Config.GuestInstallPrefix+"/bin/lima-guestagent", dev.BusID))
	if err != nil {
		if added {
			if werr := removeGrant(inst, grant, before); werr != nil {
				logrus.WithError(werr).Warn("usb: could not roll back allowlist")
			}
		}
		// The import may have succeeded before SSH failed.
		if rerr := usbip.Revoke(dev.BusID, inst.Name); rerr != nil {
			logrus.WithError(rerr).Warn("usb: could not roll back session")
		}
		return fmt.Errorf("usbip attach in guest failed: %w: %s", err, strutil.RemoveControlChars(out))
	}
	logrus.Infof("usb: attached %04x:%04x (%s) to instance %q", dev.Vendor, dev.Product, dev.BusID, inst.Name)
	return nil
}

// removeGrant restores what AddEntry replaced, so a failed attach keeps a Denied entry.
func removeGrant(inst *limatype.Instance, grant usbip.AllowEntry, before []usbip.AllowEntry) error {
	return usbip.UpdateAllowlist(inst.Dir, func(list []usbip.AllowEntry) []usbip.AllowEntry {
		list = usbip.RemoveEntry(list, grant)
		for _, e := range before {
			if len(usbip.RemoveEntry([]usbip.AllowEntry{e}, grant)) == 0 {
				list = append(list, e)
			}
		}
		return list
	})
}

func parseVIDPID(vendor, product string) (vid, pid uint16, err error) {
	if vendor == "" || product == "" {
		return 0, 0, errors.New("specify a BUSID or both --vendor and --product")
	}
	v, err := strconv.ParseUint(vendor, 16, 16)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid --vendor %q: %w", vendor, err)
	}
	p, err := strconv.ParseUint(product, 16, 16)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid --product %q: %w", product, err)
	}
	return uint16(v), uint16(p), nil
}

func resolveHostDevice(busid, vendor, product string) (usbip.DeviceInfo, error) {
	hosts, err := usbip.List()
	if err != nil {
		return usbip.DeviceInfo{}, fmt.Errorf("enumerating host USB devices: %w", err)
	}
	if busid != "" {
		for _, h := range hosts {
			if h.BusID == busid {
				return h, nil
			}
		}
		return usbip.DeviceInfo{}, fmt.Errorf("no host USB device with busid %s", busid)
	}
	vid, pid, err := parseVIDPID(vendor, product)
	if err != nil {
		return usbip.DeviceInfo{}, err
	}
	var matches []usbip.DeviceInfo
	for _, h := range hosts {
		if h.Vendor == vid && h.Product == pid {
			matches = append(matches, h)
		}
	}
	switch len(matches) {
	case 0:
		return usbip.DeviceInfo{}, fmt.Errorf("no host USB device %04x:%04x found", vid, pid)
	case 1:
		return matches[0], nil
	default:
		return usbip.DeviceInfo{}, fmt.Errorf("multiple %04x:%04x devices on host; specify a BUSID", vid, pid)
	}
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

func requireVZUSBInstance(ctx context.Context, name string) (*limatype.Instance, error) {
	inst, err := inspectVZInstance(ctx, name)
	if err != nil {
		return nil, err
	}
	if usb := inst.Config.USB; usb.Enabled == nil || !*usb.Enabled {
		return nil, fmt.Errorf("instance %q does not have USB enabled; set `usb.enabled: true` and restart", name)
	}
	if *inst.Config.Plain {
		return nil, fmt.Errorf("instance %q is in plain mode, which has no guest agent for USB passthrough", name)
	}
	if *inst.Config.OS != limatype.LINUX {
		return nil, fmt.Errorf("instance %q is not a Linux guest", name)
	}
	if inst.Status != limatype.StatusRunning {
		return nil, fmt.Errorf("instance %q is not running", name)
	}
	return inst, nil
}

func usbListComplete(cmd *cobra.Command, args []string, _ string) ([]cobra.Completion, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return bashCompleteInstanceNames(cmd)
}

func usbBusidComplete(cmd *cobra.Command, args []string, _ string) ([]cobra.Completion, cobra.ShellCompDirective) {
	if len(args) == 0 {
		return bashCompleteInstanceNames(cmd)
	}
	if len(args) > 1 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	hosts, err := usbip.List()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	heldBy := ""
	if cmd.Name() == "detach" {
		heldBy = args[0]
	}
	return busidCompletions(hosts, heldBy), cobra.ShellCompDirectiveNoFileComp
}

func busidCompletions(hosts []usbip.DeviceInfo, heldBy string) []cobra.Completion {
	var comps []cobra.Completion
	for _, h := range hosts {
		if holder, _ := usbip.Holder(h.BusID); heldBy != "" && holder != heldBy {
			continue
		}
		desc := h.ProductName
		if desc == "" {
			desc = fmt.Sprintf("%04x:%04x", h.Vendor, h.Product)
		}
		comps = append(comps, cobra.CompletionWithDesc(h.BusID, desc))
	}
	return comps
}

func runGuestCommand(ctx context.Context, inst *limatype.Instance, command string) (string, error) {
	sshExe, err := sshutil.NewSSHExe()
	if err != nil {
		return "", err
	}
	sshOpts, err := sshutil.SSHOpts(
		ctx,
		sshExe,
		inst.Dir,
		*inst.Config.User.Name,
		*inst.Config.SSH.LoadDotSSHPubKeys,
		*inst.Config.SSH.ForwardAgent,
		*inst.Config.SSH.ForwardX11,
		*inst.Config.SSH.ForwardX11Trusted)
	if err != nil {
		return "", err
	}
	sshArgs := append([]string{}, sshExe.Args...)
	sshArgs = append(sshArgs, sshutil.SSHArgsFromOpts(sshOpts)...)
	sshArgs = append(sshArgs,
		"-o", "LogLevel=ERROR",
		"-p", strconv.Itoa(inst.SSHLocalPort),
		inst.SSHAddress,
		"--",
		command,
	)
	return boundedOutput(ctx, sshExe.Exe, sshArgs...)
}

// The guest controls the output and when the command ends.
func boundedOutput(ctx context.Context, exe string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, guestTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, exe, args...)
	// A backgrounded ssh control master can hold the pipes open after exit.
	cmd.WaitDelay = time.Second
	var out cappedBuffer
	cmd.Stdout, cmd.Stderr = &out, &out
	err := cmd.Run()
	if errors.Is(err, exec.ErrWaitDelay) {
		err = nil
	}
	if ctx.Err() != nil {
		err = ctx.Err()
	}
	return string(out), err
}

type cappedBuffer []byte

func (b *cappedBuffer) Write(p []byte) (int, error) {
	*b = append(*b, p[:min(len(p), guestOutputLimit-len(*b))]...)
	return len(p), nil
}
