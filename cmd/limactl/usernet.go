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
	var hostagentCommand = &cobra.Command{
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
		if err := os.WriteFile(pidfile, []byte(strconv.Itoa(os.Getpid())+"\n"), 0644); err != nil {
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

	mtu, err := cmd.Flags().GetInt("mtu")
	if err != nil {
		return err
	}

	os.RemoveAll(endpoint)
	os.RemoveAll(qemuSocket)
	os.RemoveAll(fdSocket)

	return usernet.StartGVisorNetstack(cmd.Context(), &usernet.GVisorNetstackOpts{
		MTU:        mtu,
		Endpoint:   endpoint,
		QemuSocket: qemuSocket,
		FdSocket:   fdSocket,
		Subnet:     subnet,
	})
}
