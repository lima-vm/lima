package main

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"strings"

	"github.com/lima-vm/lima/pkg/sshutil"
	"github.com/lima-vm/lima/pkg/store"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

var copyCommand = &cli.Command{
	Name:        "copy",
	Aliases:     []string{"cp"},
	Usage:       "Copy files between host and guest",
	Description: "Prefix guest filenames with the instance name and a colon.\nExample: limactl copy default:/etc/os-release .",
	ArgsUsage:   "SOURCE ... TARGET",
	Action:      copyAction,
}

func copyAction(clicontext *cli.Context) error {
	if clicontext.NArg() < 2 {
		return errors.Errorf("requires at least 2 arguments: SOURCE DEST")
	}
	arg0, err := exec.LookPath("scp")
	if err != nil {
		return err
	}
	u, err := user.Current()
	if err != nil {
		return err
	}

	const useDotSSH = false
	args, err := sshutil.CommonArgs(useDotSSH)
	if err != nil {
		return err
	}
	args = append(args, "-3", "--")
	for _, arg := range clicontext.Args().Slice() {
		path := strings.Split(arg, ":")
		switch len(path) {
		case 1:
			args = append(args, arg)
		case 2:
			instName := path[0]
			inst, err := store.Inspect(instName)
			if err != nil {
				if errors.Is(err, os.ErrNotExist) {
					return errors.Errorf("instance %q does not exist, run `limactl start %s` to create a new instance", instName, instName)
				}
				return err
			}
			if inst.Status == store.StatusStopped {
				return errors.Errorf("instance %q is stopped, run `limactl start %s` to start the instance", instName, instName)
			}
			args = append(args, fmt.Sprintf("scp://%s@127.0.0.1:%d/%s", u.Username, inst.SSHLocalPort, path[1]))
		default:
			return errors.Errorf("Path %q contains multiple colons", arg)
		}
	}
	cmd := exec.Command(arg0, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	logrus.Debugf("executing scp (may take a long)): %+v", cmd.Args)

	// TODO: use syscall.Exec directly (results in losing tty?)
	return cmd.Run()
}
