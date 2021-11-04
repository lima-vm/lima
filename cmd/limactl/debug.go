package main

import (
	"strconv"

	"github.com/lima-vm/lima/pkg/hostagent/dns"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func newDebugCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "debug",
		Short:  "Debug utilities",
		Long:   "DO NOT USE! THE COMMAND SYNTAX IS SUBJECT TO CHANGE!",
		Hidden: true,
	}
	cmd.AddCommand(newDebugDNSCommand())
	return cmd
}

func newDebugDNSCommand() *cobra.Command {
	var cmd = &cobra.Command{
		Use:   "dns UDPPORT [TCPPORT]",
		Short: "Debug built-in DNS",
		Long:  "DO NOT USE! THE COMMAND SYNTAX IS SUBJECT TO CHANGE!",
		Args:  cobra.RangeArgs(1, 2),
		RunE:  debugDNSAction,
	}
	return cmd
}

func debugDNSAction(cmd *cobra.Command, args []string) error {
	udpLocalPort, err := strconv.Atoi(args[0])
	if err != nil {
		return err
	}
	tcpLocalPort := 0
	if len(args) > 2 {
		tcpLocalPort, err = strconv.Atoi(args[1])
		if err != nil {
			return err
		}
	}
	srv, err := dns.Start(udpLocalPort, tcpLocalPort)
	if err != nil {
		return err
	}
	logrus.Infof("Started srv %+v (UDP %d, TCP %d)", srv, udpLocalPort, tcpLocalPort)
	for {
	}
}
