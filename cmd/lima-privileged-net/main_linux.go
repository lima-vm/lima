// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"os"
	"strings"
	"unicode"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/lima-vm/lima/v2/pkg/networks"
	"github.com/lima-vm/lima/v2/pkg/osutil"
	"github.com/lima-vm/lima/v2/pkg/version"
)

func main() {
	os.Setenv("PATH", strings.Join(safeDirs, ":"))
	if err := verifyArgs(os.Args[1:]); err != nil {
		logrus.Fatal(err)
	}
	if err := newApp().Execute(); err != nil {
		osutil.HandleExitError(err)
		logrus.Fatal(err)
	}
}

// verifyArgs refuses arguments that contain whitespace. sudo matches a sudoers
// entry against the arguments joined with spaces, so the words of the entry are
// not necessarily separate arguments: "--network=host --" passed as a single
// argument satisfies an entry pinning "--network=host -- *" too, but the "--" is
// then part of the flag value instead of the end of the flags, and the words
// matched by the "*" are parsed as flags overriding the pinned ones. Without
// whitespace, the arguments are exactly the words that sudo has matched.
func verifyArgs(args []string) error {
	for _, arg := range args {
		if strings.ContainsFunc(arg, unicode.IsSpace) {
			return fmt.Errorf("argument %#q contains whitespace", arg)
		}
	}
	return nil
}

func newApp() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "lima-privileged-net",
		Short: "Do not launch manually",
		Long: `Privileged helper that manages the Linux bridges and tap devices of the
"shared", "host", and "bridged" networks. It is executed via sudo by limactl.`,
		Version:       strings.TrimPrefix(version.Version, "v"),
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(_ *cobra.Command, _ []string) error {
			return verifySelf()
		},
	}
	rootCmd.AddCommand(newStartCommand(), newStopCommand(), newTapCommand())
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

func newStopCommand() *cobra.Command {
	var pidFile string
	cmd := &cobra.Command{
		Use:   "stop",
		Short: "Terminate the `start` process that wrote the PID file",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return stop(pidFile)
		},
	}
	cmd.Flags().StringVar(&pidFile, "pidfile", "", "file the `start` process wrote its PID to")
	return cmd
}

func newTapCommand() *cobra.Command {
	var bridge, network string
	cmd := &cobra.Command{
		Use:   "tap INSTANCE_ID",
		Short: "Create a tap device owned by the calling user and attach it to a bridge",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return tapUp(cmd.Context(), bridge, network, args[0])
		},
	}
	cmd.Flags().SetInterspersed(false)
	cmd.Flags().StringVar(&bridge, "bridge", "", "name of the bridge to attach the tap device to")
	cmd.Flags().StringVar(&network, "network", "", "name of the Lima network the instance is connected to")
	return cmd
}
