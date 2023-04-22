package main

// Same as linux, but without vsock

import (
	"errors"
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
	return daemonCommand
}

func daemonAction(cmd *cobra.Command, _ []string) error {
	socket := "/run/lima-guestagent.sock"
	{ // runtime.GOOS == "freebsd"
		socket = "/var" + socket
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
	err = os.RemoveAll(socket)
	if err != nil {
		return err
	}

	var l net.Listener
	{
		socketL, err := net.Listen("unix", socket)
		if err != nil {
			return err
		}
		if err := os.Chmod(socket, 0777); err != nil {
			return err
		}
		l = socketL
		logrus.Infof("serving the guest agent on %q", socket)
	}
	return srv.Serve(l)
}
