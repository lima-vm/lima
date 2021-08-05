package main

import (
	"io"
	"os"
	"os/signal"
	"strconv"

	"github.com/lima-vm/lima/pkg/hostagent"
	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"
)

var hostagentCommand = &cli.Command{
	Name:      "hostagent",
	Usage:     "DO NOT EXECUTE MANUALLY",
	ArgsUsage: "INSTANCE",
	Hidden:    true,
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  "pidfile",
			Usage: "PID file",
		},
	},

	Action: hostagentAction,
}

func hostagentAction(clicontext *cli.Context) error {
	if pidfile := clicontext.String("pidfile"); pidfile != "" {
		if _, err := os.Stat(pidfile); !errors.Is(err, os.ErrNotExist) {
			return errors.Errorf("pidfile %q already exists", pidfile)
		}
		if err := os.WriteFile(pidfile, []byte(strconv.Itoa(os.Getpid())+"\n"), 0644); err != nil {
			return err
		}
		defer os.RemoveAll(pidfile)
	}

	if clicontext.NArg() != 1 {
		return errors.Errorf("requires exactly 1 argument")
	}

	instName := clicontext.Args().First()

	sigintCh := make(chan os.Signal, 1)
	signal.Notify(sigintCh, os.Interrupt)

	stdout := &syncWriter{w: clicontext.App.Writer}
	stderr := &syncWriter{w: clicontext.App.ErrWriter}

	ha, err := hostagent.New(instName, stdout, stderr, sigintCh)
	if err != nil {
		return err
	}
	return ha.Run(clicontext.Context)
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
