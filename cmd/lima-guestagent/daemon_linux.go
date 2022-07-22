package main

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/mux"
	"github.com/lima-vm/lima/pkg/guestagent"
	"github.com/lima-vm/lima/pkg/guestagent/api/server"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func newDaemonCommand() *cobra.Command {
	daemonCommand := &cobra.Command{
		Use:   "daemon",
		Short: "run the daemon",
		RunE:  daemonAction,
	}
	daemonCommand.Flags().Duration("tick", 3*time.Second, "tick for polling events")
	daemonCommand.Flags().Int("port", 0, "tcp port")
	return daemonCommand
}

func daemonAction(cmd *cobra.Command, args []string) error {
	unix := true
	socket := "/run/lima-guestagent.sock"
	port, err := cmd.Flags().GetInt("port")
	if err == nil && port != 0 {
		unix = false
		socket = fmt.Sprintf("127.0.0.1:%d", port)
	}
	tick, err := cmd.Flags().GetDuration("tick")
	if err != nil {
		return err
	}
	if tick == 0 {
		return errors.New("tick must be specified")
	}
	if os.Geteuid() != 0 {
		return errors.New("must run as the root")
	}
	logrus.Infof("event tick: %v", tick)

	newTicker := func() (<-chan time.Time, func()) {
		// TODO: use an equivalent of `bpftrace -e 'tracepoint:syscalls:sys_*_bind { printf("tick\n"); }')`,
		// without depending on `bpftrace` binary.
		// The agent binary will need CAP_BPF file cap.
		ticker := time.NewTicker(tick)
		return ticker.C, ticker.Stop
	}

	agent, err := guestagent.New(newTicker, tick*20)
	if err != nil {
		return err
	}
	backend := &server.Backend{
		Agent: agent,
	}
	r := mux.NewRouter()
	server.AddRoutes(r, backend)
	srv := &http.Server{Handler: r}
	var l net.Listener
	if unix {
		err = os.RemoveAll(socket)
		if err != nil {
			return err
		}
		l, err = net.Listen("unix", socket)
		if err != nil {
			return err
		}
		if err := os.Chmod(socket, 0777); err != nil {
			return err
		}
	} else {
		l, err = net.Listen("tcp4", socket)
		if err != nil {
			return err
		}
	}
	logrus.Infof("serving the guest agent on %q", socket)
	return srv.Serve(l)
}
