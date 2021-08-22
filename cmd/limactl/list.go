package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"text/tabwriter"

	"github.com/lima-vm/lima/pkg/store"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

var listCommand = &cli.Command{
	Name:    "list",
	Aliases: []string{"ls"},
	Usage:   "List instances of Lima.",
	Flags: []cli.Flag{
		&cli.BoolFlag{
			Name:  "json",
			Usage: "JSONify output",
		},
		&cli.BoolFlag{
			Name:    "quiet",
			Aliases: []string{"q"},
			Usage:   "Only show names",
		},
	},

	Action: listAction,
}

func listAction(clicontext *cli.Context) error {
	if clicontext.NArg() > 0 {
		return errors.New("too many arguments")
	}

	if clicontext.Bool("quiet") && clicontext.Bool("json") {
		return errors.New("option --quiet conflicts with --json")
	}

	instances, err := store.Instances()
	if err != nil {
		return err
	}

	if clicontext.Bool("quiet") {
		for _, instName := range instances {
			fmt.Fprintln(clicontext.App.Writer, instName)
		}
		return nil
	}

	if clicontext.Bool("json") {
		for _, instName := range instances {
			inst, err := store.Inspect(instName)
			if err != nil {
				logrus.WithError(err).Errorf("instance %q does not exist?", instName)
				continue
			}
			b, err := json.Marshal(inst)
			if err != nil {
				return err
			}
			fmt.Fprintln(clicontext.App.Writer, string(b))
		}
		return nil
	}

	w := tabwriter.NewWriter(clicontext.App.Writer, 4, 8, 4, ' ', 0)
	fmt.Fprintln(w, "NAME\tSTATUS\tSSH\tARCH\tDIR")

	if len(instances) == 0 {
		logrus.Warn("No instance found. Run `limactl start` to create an instance.")
	}

	for _, instName := range instances {
		inst, err := store.Inspect(instName)
		if err != nil {
			logrus.WithError(err).Errorf("instance %q does not exist?", instName)
			continue
		}
		if len(inst.Errors) > 0 {
			logrus.WithField("errors", inst.Errors).Warnf("instance %q has errors", instName)
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			inst.Name,
			inst.Status,
			fmt.Sprintf("127.0.0.1:%d", inst.SSHLocalPort),
			inst.Arch,
			inst.Dir,
		)
	}

	return w.Flush()
}
