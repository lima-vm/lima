package main

import (
	"errors"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/AkihiroSuda/lima/pkg/guestagent"
	"github.com/AkihiroSuda/lima/pkg/guestagent/api/server"
	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

var daemonCommand = &cli.Command{
	Name:      "daemon",
	Usage:     "run the daemon",
	ArgsUsage: "[SOCKET]",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  "socket",
			Usage: "socket",
			Value: func() string {
				if xrd := os.Getenv("XDG_RUNTIME_DIR"); xrd != "" {
					return filepath.Join(xrd, "lima-guestagent.sock")
				}
				logrus.Warn("$XDG_RUNTIME_DIR is not set, cannot determine the socket name")
				return ""
			}(),
		},
		&cli.DurationFlag{
			Name:  "tick",
			Usage: "tick for polling events",
			Value: 3 * time.Second,
		},
	},
	Action: daemonAction,
}

func daemonAction(clicontext *cli.Context) error {
	socket := clicontext.String("socket")
	if socket == "" {
		return errors.New("socket must be specified")
	}
	tick := clicontext.Duration("tick")
	if tick == 0 {
		return errors.New("tick must be specified")
	}
	if os.Geteuid() == 0 {
		return errors.New("must not run as the root")
	}
	logrus.Infof("event tick: %v", tick)

	newTicker := func() (<-chan time.Time, func()) {
		// TODO: use an equivalent of `bpftrace -e 'tracepoint:syscalls:sys_*_bind { printf("tick\n"); }')`,
		// without depending on `bpftrace` binary.
		// The agent binary will need CAP_BPF file cap.
		ticker := time.NewTicker(tick)
		return ticker.C, ticker.Stop
	}

	agent := guestagent.New(newTicker)
	backend := &server.Backend{
		Agent: agent,
	}
	r := mux.NewRouter()
	server.AddRoutes(r, backend)
	srv := &http.Server{Handler: r}
	err := os.RemoveAll(socket)
	if err != nil {
		return err
	}
	l, err := net.Listen("unix", socket)
	if err != nil {
		return err
	}
	logrus.Infof("serving the guest agent on %q", socket)
	return srv.Serve(l)
}
