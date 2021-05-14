package main

import (
	"fmt"
	"text/tabwriter"

	"github.com/AkihiroSuda/lima/pkg/store"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

var lsCommand = &cli.Command{
	Name:  "ls",
	Usage: "List instances of Lima.",
	// TODO: add --json flag for automation
	Action: lsAction,
}

func lsAction(clicontext *cli.Context) error {
	instances, err := store.Instances()
	if err != nil {
		return err
	}

	w := tabwriter.NewWriter(clicontext.App.Writer, 4, 8, 4, ' ', 0)
	fmt.Fprintln(w, "NAME\tSSH\tARCH\tDIR")

	if len(instances) == 0 {
		logrus.Warn("No instance found. Run `limactl start` to create an instance.")
	}

	for _, instName := range instances {
		y, instDir, err := store.LoadYAMLByInstanceName(instName)
		if err != nil {
			logrus.WithError(err).Warnf("failed to load instance %q", instName)
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
			instName,
			fmt.Sprintf("127.0.0.1:%d", y.SSH.LocalPort),
			y.Arch,
			instDir,
		)
	}

	return w.Flush()
}
