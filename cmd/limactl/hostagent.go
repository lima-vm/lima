package main

import (
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/lima-vm/lima/pkg/hostagent"
	"github.com/lima-vm/lima/pkg/hostagent/api/server"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func newHostagentCommand() *cobra.Command {
	var hostagentCommand = &cobra.Command{
		Use:    "hostagent INSTANCE",
		Short:  "run hostagent",
		Args:   cobra.ExactArgs(1),
		RunE:   hostagentAction,
		Hidden: true,
	}
	hostagentCommand.Flags().StringP("pidfile", "p", "", "write pid to file")
	hostagentCommand.Flags().String("socket", "", "hostagent socket")
	hostagentCommand.Flags().String("nerdctl-archive", "", "local file path (not URL) of nerdctl-full-VERSION-linux-GOARCH.tar.gz")
	return hostagentCommand
}

func hostagentAction(cmd *cobra.Command, args []string) error {
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
	socket, err := cmd.Flags().GetString("socket")
	if err != nil {
		return err
	}
	if socket == "" {
		return fmt.Errorf("socket must be specified (limactl version mismatch?)")
	}

	instName := args[0]

	sigintCh := make(chan os.Signal, 1)
	signal.Notify(sigintCh, os.Interrupt)

	stdout := &syncWriter{w: cmd.OutOrStdout()}
	stderr := &syncWriter{w: cmd.ErrOrStderr()}

	initLogrus(stderr)
	var opts []hostagent.Opt
	nerdctlArchive, err := cmd.Flags().GetString("nerdctl-archive")
	if err != nil {
		return err
	}
	if nerdctlArchive != "" {
		opts = append(opts, hostagent.WithNerdctlArchive(nerdctlArchive))
	}
	ha, err := hostagent.New(instName, stdout, sigintCh, opts...)
	if err != nil {
		return err
	}

	backend := &server.Backend{
		Agent: ha,
	}
	r := mux.NewRouter()
	server.AddRoutes(r, backend)
	srv := &http.Server{Handler: r}
	err = os.RemoveAll(socket)
	if err != nil {
		return err
	}
	l, err := net.Listen("unix", socket)
	if err != nil {
		return err
	}
	go func() {
		defer os.RemoveAll(socket)
		defer srv.Close()
		if serveErr := srv.Serve(l); serveErr != nil {
			logrus.WithError(serveErr).Warn("hostagent API server exited with an error")
		}
	}()
	return ha.Run(cmd.Context())
}

// syncer is implemented by *os.File
type syncer interface {
	Sync() error
}

type syncWriter struct {
	w io.Writer
}

func (w *syncWriter) Write(p []byte) (int, error) {
	written, err := w.w.Write(p)
	if err == nil {
		if s, ok := w.w.(syncer); ok {
			_ = s.Sync()
		}
	}
	return written, err
}

func initLogrus(stderr io.Writer) {
	logrus.SetOutput(stderr)
	// JSON logs are parsed in pkg/hostagent/events.Watcher()
	logrus.SetFormatter(new(logrus.JSONFormatter))
	logrus.SetLevel(logrus.DebugLevel)
}
