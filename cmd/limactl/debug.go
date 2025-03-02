/*
Copyright The Lima Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"strconv"
	"time"

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
	cmd := &cobra.Command{
		Use:   "dns UDPPORT [TCPPORT]",
		Short: "Debug built-in DNS",
		Long:  "DO NOT USE! THE COMMAND SYNTAX IS SUBJECT TO CHANGE!",
		Args:  WrapArgsError(cobra.RangeArgs(1, 2)),
		RunE:  debugDNSAction,
	}
	cmd.Flags().BoolP("ipv6", "6", false, "lookup IPv6 addresses too")
	return cmd
}

func debugDNSAction(cmd *cobra.Command, args []string) error {
	ipv6, err := cmd.Flags().GetBool("ipv6")
	if err != nil {
		return err
	}
	udpLocalPort, err := strconv.Atoi(args[0])
	if err != nil {
		return err
	}
	tcpLocalPort := 0
	if len(args) > 1 {
		tcpLocalPort, err = strconv.Atoi(args[1])
		if err != nil {
			return err
		}
	}
	srvOpts := dns.ServerOptions{
		UDPPort: udpLocalPort,
		TCPPort: tcpLocalPort,
		Address: "127.0.0.1",
		HandlerOptions: dns.HandlerOptions{
			IPv6:        ipv6,
			StaticHosts: map[string]string{},
		},
	}
	srv, err := dns.Start(srvOpts)
	if err != nil {
		return err
	}
	logrus.Infof("Started srv %+v (UDP %d, TCP %d)", srv, udpLocalPort, tcpLocalPort)
	for {
		time.Sleep(time.Hour)
	}
}
