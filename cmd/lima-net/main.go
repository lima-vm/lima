//go:build linux

// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/lima-vm/lima/v2/pkg/networks"
	"github.com/lima-vm/lima/v2/pkg/osutil"
	"github.com/lima-vm/lima/v2/pkg/version"
)

func main() {
	os.Setenv("PATH", strings.Join(safeDirs, ":"))
	if err := newApp().Execute(); err != nil {
		osutil.HandleExitError(err)
		logrus.Fatal(err)
	}
}

func newApp() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "lima-net",
		Short: "Do not launch manually",
		Long: `Privileged helper that manages the Linux bridges and tap devices of the
"shared", "host", and "bridged" networks. It is executed via sudo by limactl.`,
		Version:       strings.TrimPrefix(version.Version, "v"),
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	rootCmd.AddCommand(newStartCommand(), newTapCommand())
	return rootCmd
}

func newStartCommand() *cobra.Command {
	var c netConfig
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Bring a network up and stay in the foreground until terminated",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return c.run(cmd.Context())
		},
	}
	f := cmd.Flags()
	f.StringVar(&c.pidFile, "pidfile", "", "file to write the PID of this process to")
	f.StringVar(&c.mode, "mode", "", "network mode: "+networks.ModeShared+", "+networks.ModeHost+", or "+networks.ModeBridged)
	f.StringVar(&c.bridge, "bridge", "", "name of the bridge interface")
	f.StringVar(&c.gateway, "gateway", "", "IPv4 address assigned to the bridge")
	f.StringVar(&c.netmask, "netmask", "", "IPv4 netmask of the network")
	f.StringVar(&c.dhcpEnd, "dhcp-end", "", "last IPv4 address handed out by DHCP")
	return cmd
}

func newTapCommand() *cobra.Command {
	var bridge string
	cmd := &cobra.Command{
		Use:   "tap TAP",
		Short: "Create a tap device owned by the calling user and attach it to a bridge",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return tapUp(cmd.Context(), args[0], bridge)
		},
	}
	cmd.Flags().StringVar(&bridge, "bridge", "", "name of the bridge to attach the tap device to")
	return cmd
}
