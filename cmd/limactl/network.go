// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net"
	"os"
	"slices"
	"strings"
	"text/tabwriter"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/lima-vm/lima/v2/pkg/networks"
	"github.com/lima-vm/lima/v2/pkg/yqutil"
)

const networkCreateExample = `  Create a network:
  $ limactl network create foo --gateway 192.168.42.1/24

  Connect VM instances to the newly created network:
  $ limactl create --network lima:foo --name vm1
  $ limactl create --network lima:foo --name vm2
`

func newNetworkCommand() *cobra.Command {
	networkCommand := &cobra.Command{
		Use:     "network",
		Short:   "Lima network management",
		Example: networkCreateExample,
		GroupID: advancedCommand,
	}
	networkCommand.AddCommand(
		newNetworkListCommand(),
		newNetworkCreateCommand(),
		newNetworkDeleteCommand(),
	)
	return networkCommand
}

func newNetworkListCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "list",
		Short:             "List networks",
		Aliases:           []string{"ls"},
		Args:              WrapArgsError(cobra.ArbitraryArgs),
		RunE:              networkListAction,
		ValidArgsFunction: networkBashComplete,
	}
	flags := cmd.Flags()
	flags.Bool("json", false, "JSONify output")
	return cmd
}

func networkListAction(cmd *cobra.Command, args []string) error {
	flags := cmd.Flags()
	jsonFormat, err := flags.GetBool("json")
	if err != nil {
		return err
	}

	config, err := networks.LoadConfig()
	if err != nil {
		return err
	}

	allNetworks := slices.Sorted(maps.Keys(config.Networks))

	networks := []string{}
	if len(args) > 0 {
		for _, arg := range args {
			matches := nameMatches(arg, allNetworks)
			if len(matches) > 0 {
				networks = append(networks, matches...)
			} else {
				logrus.Warnf("No network matching %v found.", arg)
			}
		}
	} else {
		networks = allNetworks
	}

	if jsonFormat {
		w := cmd.OutOrStdout()
		for _, name := range networks {
			nw, ok := config.Networks[name]
			if !ok {
				logrus.Errorf("network %q does not exist", nw)
				continue
			}
			j, err := json.Marshal(nw)
			if err != nil {
				return err
			}
			fmt.Fprintln(w, string(j))
		}
		return nil
	}

	w := tabwriter.NewWriter(cmd.OutOrStdout(), 4, 8, 4, ' ', 0)
	fmt.Fprintln(w, "NAME\tMODE\tGATEWAY\tINTERFACE")
	for _, name := range networks {
		nw, ok := config.Networks[name]
		if !ok {
			logrus.Errorf("network %q does not exist", nw)
			continue
		}
		gwStr := "-"
		if nw.Gateway != nil {
			gw := net.IPNet{
				IP:   nw.Gateway,
				Mask: net.IPMask(nw.NetMask),
			}
			gwStr = gw.String()
		}
		intfStr := "-"
		if nw.Interface != "" {
			intfStr = nw.Interface
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", name, nw.Mode, gwStr, intfStr)
	}
	return w.Flush()
}

func newNetworkCreateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "create NETWORK",
		Short:   "Create a Lima network",
		Example: networkCreateExample,
		Args:    WrapArgsError(cobra.ExactArgs(1)),
		RunE:    networkCreateAction,
	}
	flags := cmd.Flags()
	flags.String("mode", networks.ModeUserV2, "mode")
	_ = cmd.RegisterFlagCompletionFunc("mode", func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
		return networks.Modes, cobra.ShellCompDirectiveNoFileComp
	})
	flags.String("gateway", "", "gateway, e.g., \"192.168.42.1/24\"")
	flags.String("interface", "", "interface for bridged mode")
	_ = cmd.RegisterFlagCompletionFunc("interface", bashFlagCompleteNetworkInterfaceNames)
	return cmd
}

func networkCreateAction(cmd *cobra.Command, args []string) error {
	name := args[0]
	// LoadConfig ensures existence of networks.yaml
	config, err := networks.LoadConfig()
	if err != nil {
		return err
	}
	if _, ok := config.Networks[name]; ok {
		return fmt.Errorf("network %q already exists", name)
	}

	flags := cmd.Flags()
	mode, err := flags.GetString("mode")
	if err != nil {
		return err
	}

	gateway, err := flags.GetString("gateway")
	if err != nil {
		return err
	}

	intf, err := flags.GetString("interface")
	if err != nil {
		return err
	}

	ctx := cmd.Context()

	switch mode {
	case networks.ModeBridged:
		if gateway != "" {
			return fmt.Errorf("network mode %q does not support specifying gateway", mode)
		}
		if intf == "" {
			return fmt.Errorf("network mode %q requires specifying interface", mode)
		}
		yq := fmt.Sprintf(`.networks.%q = {"mode":%q,"interface":%q}`, name, mode, intf)
		return networkApplyYQ(ctx, yq)
	default:
		if gateway == "" {
			return fmt.Errorf("network mode %q requires specifying gateway", mode)
		}
		if intf != "" {
			return fmt.Errorf("network mode %q does not support specifying interface", mode)
		}
		if !strings.Contains(gateway, "/") {
			gateway += "/24"
		}
		gwIP, gwMask, err := net.ParseCIDR(gateway)
		if err != nil {
			return fmt.Errorf("failed to parse CIDR %q: %w", gateway, err)
		}
		if gwIP.IsUnspecified() || gwIP.IsLoopback() {
			return fmt.Errorf("invalid IP address: %v", gwIP)
		}
		gwMaskStr := "255.255.255.0"
		if gwMask != nil {
			gwMaskStr = net.IP(gwMask.Mask).String()
		}
		// TODO: check IP range collision

		yq := fmt.Sprintf(`.networks.%q = {"mode":%q,"gateway":%q,"netmask":%q,"interface":%q}`, name, mode, gwIP.String(), gwMaskStr, intf)
		return networkApplyYQ(ctx, yq)
	}
}

func networkApplyYQ(ctx context.Context, yq string) error {
	filePath, err := networks.ConfigFile()
	if err != nil {
		return err
	}
	yContent, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}
	yBytes, err := yqutil.EvaluateExpression(ctx, yq, yContent)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filePath, yBytes, 0o644); err != nil {
		return err
	}
	return nil
}

func newNetworkDeleteCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "delete NETWORK [NETWORK, ...]",
		Short:             "Delete one or more Lima networks",
		Aliases:           []string{"remove", "rm"},
		Args:              WrapArgsError(cobra.MinimumNArgs(1)),
		RunE:              networkDeleteAction,
		ValidArgsFunction: networkBashComplete,
	}
	flags := cmd.Flags()
	flags.BoolP("force", "f", false, "Force delete (currently always required)")
	return cmd
}

func networkDeleteAction(cmd *cobra.Command, args []string) error {
	flags := cmd.Flags()
	force, err := flags.GetBool("force")
	if err != nil {
		return err
	}
	if !force {
		return errors.New("`limactl network delete` currently always requires `--force`")
		// Because the command currently does not check whether the network being removed is in use
	}

	var yq string
	for i, name := range args {
		yq += fmt.Sprintf("del(.networks.%q)", name)
		if i < len(args)-1 {
			yq += " | "
		}
	}
	return networkApplyYQ(cmd.Context(), yq)
}

func networkBashComplete(cmd *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	return bashCompleteNetworkNames(cmd)
}
