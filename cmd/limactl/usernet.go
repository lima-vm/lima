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
	"errors"
	"fmt"
	"os"
	"strconv"

	"github.com/lima-vm/lima/pkg/networks/usernet"
	"github.com/spf13/cobra"
)

func newUsernetCommand() *cobra.Command {
	hostagentCommand := &cobra.Command{
		Use:    "usernet",
		Short:  "run usernet",
		Args:   cobra.ExactArgs(0),
		RunE:   usernetAction,
		Hidden: true,
	}
	hostagentCommand.Flags().StringP("pidfile", "p", "", "write pid to file")
	hostagentCommand.Flags().StringP("endpoint", "e", "", "exposes usernet api(s) on this endpoint")
	hostagentCommand.Flags().String("listen-qemu", "", "listen for qemu connections")
	hostagentCommand.Flags().String("listen", "", "listen on a Unix socket and receive Bess-compatible FDs as SCM_RIGHTS messages")
	hostagentCommand.Flags().String("subnet", "192.168.5.0/24", "sets subnet value for the usernet network")
	hostagentCommand.Flags().Int("mtu", 1500, "mtu")
	hostagentCommand.Flags().StringToString("leases", nil, "pass default static leases for startup. Eg: '192.168.104.1=52:55:55:b3:bc:d9,192.168.104.2=5a:94:ef:e4:0c:df' ")
	return hostagentCommand
}

func usernetAction(cmd *cobra.Command, _ []string) error {
	pidfile, err := cmd.Flags().GetString("pidfile")
	if err != nil {
		return err
	}
	if pidfile != "" {
		if _, err := os.Stat(pidfile); !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("pidfile %q already exists", pidfile)
		}
		if err := os.WriteFile(pidfile, []byte(strconv.Itoa(os.Getpid())+"\n"), 0o644); err != nil {
			return err
		}
		defer os.RemoveAll(pidfile)
	}
	endpoint, err := cmd.Flags().GetString("endpoint")
	if err != nil {
		return err
	}
	qemuSocket, err := cmd.Flags().GetString("listen-qemu")
	if err != nil {
		return err
	}
	fdSocket, err := cmd.Flags().GetString("listen")
	if err != nil {
		return err
	}
	subnet, err := cmd.Flags().GetString("subnet")
	if err != nil {
		return err
	}

	leases, err := cmd.Flags().GetStringToString("leases")
	if err != nil {
		return err
	}

	mtu, err := cmd.Flags().GetInt("mtu")
	if err != nil {
		return err
	}

	os.RemoveAll(endpoint)
	os.RemoveAll(qemuSocket)
	os.RemoveAll(fdSocket)

	// Environment Variables
	// LIMA_USERNET_RESOLVE_IP_ADDRESS_TIMEOUT: Specifies the timeout duration for resolving IP addresses in minutes. Default is 2 minutes.

	return usernet.StartGVisorNetstack(cmd.Context(), &usernet.GVisorNetstackOpts{
		MTU:           mtu,
		Endpoint:      endpoint,
		QemuSocket:    qemuSocket,
		FdSocket:      fdSocket,
		Subnet:        subnet,
		DefaultLeases: leases,
	})
}
