package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strconv"

	"github.com/lima-vm/lima/pkg/hostagent"
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

	instName := args[0]

	sigintCh := make(chan os.Signal, 1)
	signal.Notify(sigintCh, os.Interrupt)

	stdout := &syncWriter{w: cmd.OutOrStdout()}
	stderr := &syncWriter{w: cmd.ErrOrStderr()}

	ha, err := hostagent.New(instName, stdout, stderr, sigintCh)
	if err != nil {
		return err
	}
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
