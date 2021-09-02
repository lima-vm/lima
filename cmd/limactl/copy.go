package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"strings"

	"github.com/lima-vm/lima/pkg/sshutil"
	"github.com/lima-vm/lima/pkg/store"
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
		return fmt.Errorf("requires at least 2 arguments: SOURCE DEST")
	}
	arg0, err := exec.LookPath("scp")
	if err != nil {
		return err
	}
	u, err := user.Current()
	if err != nil {
		return err
	}

	instDirs := make(map[string]string)
	args := []string{"-3", "--"}
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
					return fmt.Errorf("instance %q does not exist, run `limactl start %s` to create a new instance", instName, instName)
				}
				return err
			}
			if inst.Status == store.StatusStopped {
				return fmt.Errorf("instance %q is stopped, run `limactl start %s` to start the instance", instName, instName)
			}
			args = append(args, fmt.Sprintf("scp://%s@127.0.0.1:%d/%s", u.Username, inst.SSHLocalPort, path[1]))
			instDirs[instName] = inst.Dir
		default:
			return fmt.Errorf("Path %q contains multiple colons", arg)
		}
	}

	sshArgs := []string{}
	if len(instDirs) == 1 {
		// Only one (instance) host is involved; we can use the instance-specific
		// arguments such as ControlPath.  This is preferred as we can multiplex
		// sessions without re-authenticating (MaxSessions permitting).
		for _, instDir := range instDirs {
			sshArgs, err = sshutil.SSHArgs(instDir, false)
			if err != nil {
				return err
			}
		}
	} else {
		// Copying among multiple hosts; we can't pass in host-specific options.
		sshArgs, err = sshutil.CommonArgs(false)
		if err != nil {
			return err
		}
	}

	cmd := exec.Command(arg0, append(sshArgs, args...)...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	logrus.Debugf("executing scp (may take a long time)): %+v", cmd.Args)

	// TODO: use syscall.Exec directly (results in losing tty?)
	return cmd.Run()
}
