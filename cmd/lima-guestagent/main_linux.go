package main

import (
	"os"
	"strings"

	"github.com/AkihiroSuda/lima/pkg/version"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

func main() {
	if err := newApp().Run(os.Args); err != nil {
		logrus.Fatal(err)
	}
}

func newApp() *cli.App {
	app := cli.NewApp()
	app.Name = "lima-guestagent"
	app.Usage = "Do not launch manually"
	app.Version = strings.TrimPrefix(version.Version, "v")
	app.Flags = []cli.Flag{
		&cli.BoolFlag{
			Name:  "debug",
			Usage: "debug mode",
		},
	}
	app.Before = func(clicontext *cli.Context) error {
		if clicontext.Bool("debug") {
			logrus.SetLevel(logrus.DebugLevel)
		}
		return nil
	}
	app.Commands = []*cli.Command{
		daemonCommand,
	}
	return app
}
