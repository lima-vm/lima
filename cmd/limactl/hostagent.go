package main

import (
	"os"
	"os/signal"
	"strconv"

	"github.com/AkihiroSuda/lima/pkg/hostagent"
	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"
)

var hostagentCommand = &cli.Command{
	Name:      "hostagent",
	Usage:     "DO NOT EXECUTE MANUALLY",
	ArgsUsage: "NAME",
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

	stdout := clicontext.App.Writer
	stderr := clicontext.App.ErrWriter

	ha, err := hostagent.New(instName, stdout, stderr, sigintCh)
	if err != nil {
		return err
	}
	return ha.Run(clicontext.Context)
}
