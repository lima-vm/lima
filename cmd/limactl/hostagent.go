// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"strconv"
	"syscall"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/lima-vm/lima/v2/pkg/hostagent"
	"github.com/lima-vm/lima/v2/pkg/hostagent/api/server"
)

func newHostagentCommand() *cobra.Command {
	hostagentCommand := &cobra.Command{
		Use:    "hostagent INSTANCE",
		Short:  "Run hostagent",
		Args:   WrapArgsError(cobra.ExactArgs(1)),
		RunE:   hostagentAction,
		Hidden: true,
	}
	hostagentCommand.Flags().StringP("pidfile", "p", "", "Write PID to file")
	hostagentCommand.Flags().String("socket", "", "Path of hostagent socket")
	hostagentCommand.Flags().Bool("run-gui", false, "Run GUI synchronously within hostagent")
	hostagentCommand.Flags().String("guestagent", "", "Local file path (not URL) of lima-guestagent.OS-ARCH[.gz]")
	hostagentCommand.Flags().String("nerdctl-archive", "", "Local file path (not URL) of nerdctl-full-VERSION-GOOS-GOARCH.tar.gz")
	hostagentCommand.Flags().Bool("progress", false, "Show provision script progress by monitoring cloud-init logs")
	return hostagentCommand
}

func hostagentAction(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
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
	socket, err := cmd.Flags().GetString("socket")
	if err != nil {
		return err
	}
	if socket == "" {
		return errors.New("socket must be specified (limactl version mismatch?)")
	}

	instName := args[0]

	runGUI, err := cmd.Flags().GetBool("run-gui")
	if err != nil {
		return err
	}
	if runGUI {
		// Without this the call to vz.RunGUI fails. Adding it here, as this has to be called before the vz cgo loads.
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
	}

	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, os.Interrupt, syscall.SIGTERM)

	stdout := &syncWriter{w: cmd.OutOrStdout()}
	stderr := &syncWriter{w: cmd.ErrOrStderr()}

	initLogrus(stderr)
	var opts []hostagent.Opt
	guestagentBinary, err := cmd.Flags().GetString("guestagent")
	if err != nil {
		return err
	}
	if guestagentBinary != "" {
		opts = append(opts, hostagent.WithGuestAgentBinary(guestagentBinary))
	}
	nerdctlArchive, err := cmd.Flags().GetString("nerdctl-archive")
	if err != nil {
		return err
	}
	if nerdctlArchive != "" {
		opts = append(opts, hostagent.WithNerdctlArchive(nerdctlArchive))
	}
	showProgress, err := cmd.Flags().GetBool("progress")
	if err != nil {
		return err
	}
	if showProgress {
		opts = append(opts, hostagent.WithCloudInitProgress(showProgress))
	}
	ha, err := hostagent.New(ctx, instName, stdout, signalCh, opts...)
	if err != nil {
		return err
	}

	backend := &server.Backend{
		Agent: ha,
	}
	r := http.NewServeMux()
	server.AddRoutes(r, backend)
	srv := &http.Server{Handler: r}
	err = os.RemoveAll(socket)
	if err != nil {
		return err
	}
	var lc net.ListenConfig
	l, err := lc.Listen(ctx, "unix", socket)
	logrus.Infof("hostagent socket created at %s", socket)
	if err != nil {
		return err
	}
	go func() {
		defer os.RemoveAll(socket)
		defer srv.Close()
		if serveErr := srv.Serve(l); serveErr != http.ErrServerClosed {
			logrus.WithError(serveErr).Warn("hostagent API server exited with an error")
		}
	}()
	return ha.Run(cmd.Context())
}

// syncer is implemented by *os.File.
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
	// HostAgent logging is one level more verbose than the start command itself
	if logrus.GetLevel() == logrus.DebugLevel {
		logrus.SetLevel(logrus.TraceLevel)
	} else {
		logrus.SetLevel(logrus.DebugLevel)
	}
}
