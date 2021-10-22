package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/lima-vm/lima/pkg/osutil"
	"github.com/lima-vm/lima/pkg/sshutil"
	"github.com/lima-vm/lima/pkg/store"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var copyHelp = `Copy files between host and guest

Prefix guest filenames with the instance name and a colon.

Example: limactl copy default:/etc/os-release .
`

func newCopyCommand() *cobra.Command {
	var copyCommand = &cobra.Command{
		Use:     "copy SOURCE ... TARGET",
		Aliases: []string{"cp"},
		Short:   "Copy files between host and guest",
		Long:    copyHelp,
		Args:    cobra.MinimumNArgs(2),
		RunE:    copyAction,
	}

	copyCommand.Flags().BoolP("recursive", "r", false, "copy directories recursively")

	return copyCommand
}

func copyAction(cmd *cobra.Command, args []string) error {
	recursive, err := cmd.Flags().GetBool("recursive")
	if err != nil {
		return err
	}

	arg0, err := exec.LookPath("scp")
	if err != nil {
		return err
	}
	u, err := osutil.LimaUser(false)
	if err != nil {
		return err
	}
	instDirs := make(map[string]string)
	scpArgs := []string{}
	debug, err := cmd.Flags().GetBool("debug")
	if err != nil {
		return err
	}
	if debug {
		scpArgs = append(scpArgs, "-v")
	}
	if recursive {
		scpArgs = append(scpArgs, "-r")
	}
	scpArgs = append(scpArgs, "-3", "--")
	for _, arg := range args {
		path := strings.Split(arg, ":")
		switch len(path) {
		case 1:
			scpArgs = append(scpArgs, arg)
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
			scpArgs = append(scpArgs, fmt.Sprintf("scp://%s@127.0.0.1:%d/%s", u.Username, inst.SSHLocalPort, path[1]))
			instDirs[instName] = inst.Dir
		default:
			return fmt.Errorf("path %q contains multiple colons", arg)
		}
	}

	var sshOpts []string
	if len(instDirs) == 1 {
		// Only one (instance) host is involved; we can use the instance-specific
		// arguments such as ControlPath.  This is preferred as we can multiplex
		// sessions without re-authenticating (MaxSessions permitting).
		for _, instDir := range instDirs {
			sshOpts, err = sshutil.SSHOpts(instDir, false, false)
			if err != nil {
				return err
			}
		}
	} else {
		// Copying among multiple hosts; we can't pass in host-specific options.
		sshOpts, err = sshutil.CommonOpts(false)
		if err != nil {
			return err
		}
	}
	sshArgs := sshutil.SSHArgsFromOpts(sshOpts)

	sshCmd := exec.Command(arg0, append(sshArgs, scpArgs...)...)
	sshCmd.Stdin = cmd.InOrStdin()
	sshCmd.Stdout = cmd.OutOrStdout()
	sshCmd.Stderr = cmd.ErrOrStderr()
	logrus.Debugf("executing scp (may take a long time)): %+v", sshCmd.Args)

	// TODO: use syscall.Exec directly (results in losing tty?)
	return sshCmd.Run()
}
