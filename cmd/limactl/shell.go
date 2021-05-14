package main

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/AkihiroSuda/lima/pkg/sshutil"
	"github.com/AkihiroSuda/lima/pkg/store"
	"github.com/alessio/shellescape"
	"github.com/mattn/go-isatty"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

var shellCommand = &cli.Command{
	Name:      "shell",
	Usage:     "Execute shell in Lima",
	ArgsUsage: "[flags] INSTANCE [COMMAND...]",
	Description: "`lima` command is provided as an alias for `limactl shell $LIMA_INSTANCE`. $LIMA_INSTANCE defaults to " + DefaultInstanceName + ".\n" +
		"Hint: try --debug to show the detailed logs, if it seems hanging (mostly due to some SSH issue).",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  "workdir",
			Usage: "working directory",
			Value: func() string {
				wd, err := os.Getwd()
				if err != nil {
					logrus.WithError(err).Warn("failed to get the current directory")
					home, err := os.UserHomeDir()
					if err != nil {
						logrus.WithError(err).Warn("failed to get the home directory")
						return "/"
					}
					return home
				}
				return wd
			}(),
		},
	},
	Action:       shellAction,
	BashComplete: shellBashComplete,
}

func shellAction(clicontext *cli.Context) error {
	if clicontext.NArg() == 0 {
		return errors.Errorf("requires at least 1 argument")
	}
	instName := clicontext.Args().First()

	switch clicontext.Args().Get(1) {
	case "start", "delete", "shell":
		// `lima start` (alias of `limactl $LIMA_INSTANCE start`) is probably a typo of `limactl start`
		logrus.Warnf("Perhaps you meant `limactl %s`?", strings.Join(clicontext.Args().Slice(), " "))
	}

	hostHome, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	script := fmt.Sprintf(" cd %q || cd %q ; exec bash --login", clicontext.String("workdir"), hostHome)
	if clicontext.NArg() > 1 {
		script += fmt.Sprintf(" -c %q", shellescape.QuoteCommand(clicontext.Args().Tail()))
	}

	y, instDir, err := store.LoadYAMLByInstanceName(instName)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return errors.Errorf("instance %q does not exist, run `limactl start %s` to create a new instance", instName, instName)
		}
		return err
	}

	arg0, err := exec.LookPath("ssh")
	if err != nil {
		return err
	}

	args, err := sshutil.SSHArgs(instDir)
	if err != nil {
		return err
	}
	if isatty.IsTerminal(os.Stdout.Fd()) {
		// required for showing the shell prompt: https://stackoverflow.com/a/626574
		args = append(args, "-t")
	}
	args = append(args, []string{
		"-q",
		"-p", strconv.Itoa(y.SSH.LocalPort),
		"127.0.0.1",
		"--",
		script,
	}...)
	cmd := exec.Command(arg0, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	logrus.Debugf("executing ssh (may take a long)): %+v", cmd.Args)

	// TODO: use syscall.Exec directly (results in losing tty?)
	return cmd.Run()
}

func shellBashComplete(clicontext *cli.Context) {
	bashCompleteInstanceNames(clicontext)
}
