// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/coreos/go-semver/semver"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/lima-vm/lima/v2/pkg/ioutilx"
	"github.com/lima-vm/lima/v2/pkg/limatype"
	"github.com/lima-vm/lima/v2/pkg/sshutil"
	"github.com/lima-vm/lima/v2/pkg/store"
)

const copyHelp = `Copy files between host and guest

Prefix guest filenames with the instance name and a colon.

Backends:
  auto   - Automatically selects the best available backend (rsync preferred, falls back to scp)
  rsync  - Uses rsync for faster transfers with resume capability (requires rsync on both host and guest)
  scp    - Uses scp for reliable transfers (always available)

Examples:
  # Copy file from guest to host (auto backend)
  limactl copy default:/etc/os-release .

  # Copy file from host to guest with verbose output
  limactl copy -v myfile.txt default:/tmp/

  # Copy directory recursively using rsync backend
  limactl copy --backend=rsync -r ./mydir default:/tmp/

  # Copy using scp backend specifically
  limactl copy --backend=scp default:/var/log/app.log ./logs/

  # Copy multiple files
  limactl copy file1.txt file2.txt default:/tmp/

Not to be confused with 'limactl clone'.
`

type copyTool string

const (
	rsync copyTool = "rsync"
	scp   copyTool = "scp"
	auto  copyTool = "auto"
)

type copyPath struct {
	instanceName string
	path         string
	isRemote     bool
	instance     *limatype.Instance
}

func newCopyCommand() *cobra.Command {
	copyCommand := &cobra.Command{
		Use:     "copy SOURCE ... TARGET",
		Aliases: []string{"cp"},
		Short:   "Copy files between host and guest",
		Long:    copyHelp,
		Args:    WrapArgsError(cobra.MinimumNArgs(2)),
		RunE:    copyAction,
		GroupID: advancedCommand,
	}

	copyCommand.Flags().BoolP("recursive", "r", false, "Copy directories recursively")
	copyCommand.Flags().BoolP("verbose", "v", false, "Enable verbose output")
	copyCommand.Flags().String("backend", "auto", "Copy backend (scp|rsync|auto)")

	return copyCommand
}

func copyAction(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	recursive, err := cmd.Flags().GetBool("recursive")
	if err != nil {
		return err
	}

	verbose, err := cmd.Flags().GetBool("verbose")
	if err != nil {
		return err
	}

	debug, err := cmd.Flags().GetBool("debug")
	if err != nil {
		return err
	}

	if debug {
		verbose = true
	}

	copyPaths, err := parseCopyArgs(ctx, args)
	if err != nil {
		return err
	}

	backend, err := cmd.Flags().GetString("backend")
	if err != nil {
		return err
	}

	cpTool, toolPath, err := selectCopyTool(ctx, copyPaths, backend)
	if err != nil {
		return err
	}

	logrus.Debugf("using copy tool %q", toolPath)

	var copyCmd *exec.Cmd
	switch cpTool {
	case scp:
		copyCmd, err = scpCommand(ctx, toolPath, copyPaths, verbose, recursive)
	case rsync:
		copyCmd, err = rsyncCommand(ctx, toolPath, copyPaths, verbose, recursive)
	default:
		err = fmt.Errorf("invalid copy tool %q", cpTool)
	}
	if err != nil {
		return err
	}

	copyCmd.Stdin = cmd.InOrStdin()
	copyCmd.Stdout = cmd.OutOrStdout()
	copyCmd.Stderr = cmd.ErrOrStderr()
	logrus.Debugf("executing %v (may take a long time)", copyCmd)

	// TODO: use syscall.Exec directly (results in losing tty?)
	return copyCmd.Run()
}

func parseCopyArgs(ctx context.Context, args []string) ([]*copyPath, error) {
	var copyPaths []*copyPath

	for _, arg := range args {
		cp := &copyPath{}

		if runtime.GOOS == "windows" {
			if filepath.IsAbs(arg) {
				var err error
				arg, err = ioutilx.WindowsSubsystemPath(ctx, arg)
				if err != nil {
					return nil, err
				}
			} else {
				arg = filepath.ToSlash(arg)
			}
		}

		parts := strings.SplitN(arg, ":", 2)
		switch len(parts) {
		case 1:
			cp.path = arg
			cp.isRemote = false
		case 2:
			cp.instanceName = parts[0]
			cp.path = parts[1]
			cp.isRemote = true

			inst, err := store.Inspect(ctx, cp.instanceName)
			if err != nil {
				if errors.Is(err, os.ErrNotExist) {
					return nil, fmt.Errorf("instance %q does not exist, run `limactl create %s` to create a new instance", cp.instanceName, cp.instanceName)
				}
				return nil, err
			}
			if inst.Status == limatype.StatusStopped {
				return nil, fmt.Errorf("instance %q is stopped, run `limactl start %s` to start the instance", cp.instanceName, cp.instanceName)
			}
			cp.instance = inst
		default:
			return nil, fmt.Errorf("path %q contains multiple colons", arg)
		}

		copyPaths = append(copyPaths, cp)
	}

	return copyPaths, nil
}

func selectCopyTool(ctx context.Context, copyPaths []*copyPath, backend string) (copyTool, string, error) {
	switch copyTool(backend) {
	case scp:
		scpPath, err := exec.LookPath("scp")
		if err != nil {
			return "", "", fmt.Errorf("scp not found on host: %w", err)
		}
		return scp, scpPath, nil
	case rsync:
		rsyncPath, err := exec.LookPath("rsync")
		if err != nil {
			return "", "", fmt.Errorf("rsync not found on host: %w", err)
		}
		if !rsyncAvailableOnGuests(ctx, copyPaths) {
			return "", "", errors.New("rsync not available on guest(s)")
		}
		return rsync, rsyncPath, nil
	case auto:
		if rsyncPath, err := exec.LookPath("rsync"); err == nil {
			if rsyncAvailableOnGuests(ctx, copyPaths) {
				return rsync, rsyncPath, nil
			}
			logrus.Debugf("rsync not available on guest(s), falling back to scp")
		} else {
			logrus.Debugf("rsync not found on host, falling back to scp: %v", err)
		}

		scpPath, err := exec.LookPath("scp")
		if err != nil {
			return "", "", fmt.Errorf("neither rsync nor scp found on host: %w", err)
		}
		return scp, scpPath, nil
	default:
		return "", "", fmt.Errorf("invalid backend %q, must be one of: scp, rsync, auto", backend)
	}
}

func rsyncAvailableOnGuests(ctx context.Context, copyPaths []*copyPath) bool {
	instances := make(map[string]*limatype.Instance)

	for _, cp := range copyPaths {
		if cp.isRemote {
			instances[cp.instanceName] = cp.instance
		}
	}

	for instName, inst := range instances {
		if !checkRsyncOnGuest(ctx, inst) {
			logrus.Debugf("rsync not available on instance %q", instName)
			return false
		}
	}

	return true
}

func checkRsyncOnGuest(ctx context.Context, inst *limatype.Instance) bool {
	sshExe, err := sshutil.NewSSHExe()
	if err != nil {
		logrus.Debugf("failed to create SSH executable: %v", err)
		return false
	}
	sshOpts, err := sshutil.SSHOpts(ctx, sshExe, inst.Dir, *inst.Config.User.Name, false, false, false, false)
	if err != nil {
		logrus.Debugf("failed to get SSH options for rsync check: %v", err)
		return false
	}

	sshArgs := sshutil.SSHArgsFromOpts(sshOpts)
	checkCmd := exec.CommandContext(ctx, "ssh")
	checkCmd.Args = append(checkCmd.Args, sshArgs...)
	checkCmd.Args = append(checkCmd.Args,
		"-p", fmt.Sprintf("%d", inst.SSHLocalPort),
		fmt.Sprintf("%s@127.0.0.1", *inst.Config.User.Name),
		"command -v rsync >/dev/null 2>&1",
	)

	err = checkCmd.Run()
	return err == nil
}

func scpCommand(ctx context.Context, command string, copyPaths []*copyPath, verbose, recursive bool) (*exec.Cmd, error) {
	instances := make(map[string]*limatype.Instance)
	scpFlags := []string{}
	scpArgs := []string{}

	if verbose {
		scpFlags = append(scpFlags, "-v")
	} else {
		scpFlags = append(scpFlags, "-q")
	}

	if recursive {
		scpFlags = append(scpFlags, "-r")
	}

	// this assumes that ssh and scp come from the same place, but scp has no -V
	sshExeForVersion, err := sshutil.NewSSHExe()
	if err != nil {
		return nil, err
	}
	legacySSH := sshutil.DetectOpenSSHVersion(ctx, sshExeForVersion).LessThan(*semver.New("8.0.0"))

	for _, cp := range copyPaths {
		if cp.isRemote {
			if legacySSH {
				scpFlags = append(scpFlags, "-P", fmt.Sprintf("%d", cp.instance.SSHLocalPort))
				scpArgs = append(scpArgs, fmt.Sprintf("%s@127.0.0.1:%s", *cp.instance.Config.User.Name, cp.path))
			} else {
				scpArgs = append(scpArgs, fmt.Sprintf("scp://%s@127.0.0.1:%d/%s", *cp.instance.Config.User.Name, cp.instance.SSHLocalPort, cp.path))
			}
			instances[cp.instanceName] = cp.instance
		} else {
			scpArgs = append(scpArgs, cp.path)
		}
	}

	if legacySSH && len(instances) > 1 {
		return nil, errors.New("more than one (instance) host is involved in this command, this is only supported for openSSH v8.0 or higher")
	}

	scpFlags = append(scpFlags, "-3", "--")
	scpArgs = append(scpFlags, scpArgs...)

	var sshOpts []string
	if len(instances) == 1 {
		// Only one (instance) host is involved; we can use the instance-specific
		// arguments such as ControlPath.  This is preferred as we can multiplex
		// sessions without re-authenticating (MaxSessions permitting).
		for _, inst := range instances {
			sshExe, err := sshutil.NewSSHExe()
			if err != nil {
				return nil, err
			}
			sshOpts, err = sshutil.SSHOpts(ctx, sshExe, inst.Dir, *inst.Config.User.Name, false, false, false, false)
			if err != nil {
				return nil, err
			}
		}
	} else {
		// Copying among multiple hosts; we can't pass in host-specific options.
		sshExe, err := sshutil.NewSSHExe()
		if err != nil {
			return nil, err
		}
		sshOpts, err = sshutil.CommonOpts(ctx, sshExe, false)
		if err != nil {
			return nil, err
		}
	}
	sshArgs := sshutil.SSHArgsFromOpts(sshOpts)

	return exec.CommandContext(ctx, command, append(sshArgs, scpArgs...)...), nil
}

func rsyncCommand(ctx context.Context, command string, copyPaths []*copyPath, verbose, recursive bool) (*exec.Cmd, error) {
	rsyncFlags := []string{"-a"}

	if verbose {
		rsyncFlags = append(rsyncFlags, "-v", "--progress")
	} else {
		rsyncFlags = append(rsyncFlags, "-q")
	}

	if recursive {
		rsyncFlags = append(rsyncFlags, "-r")
	}

	rsyncArgs := make([]string, 0, len(rsyncFlags)+len(copyPaths))
	rsyncArgs = append(rsyncArgs, rsyncFlags...)

	var sshCmd string
	var remoteInstance *limatype.Instance

	for _, cp := range copyPaths {
		if cp.isRemote {
			if remoteInstance == nil {
				remoteInstance = cp.instance
				sshExe, err := sshutil.NewSSHExe()
				if err != nil {
					return nil, err
				}
				sshOpts, err := sshutil.SSHOpts(ctx, sshExe, cp.instance.Dir, *cp.instance.Config.User.Name, false, false, false, false)
				if err != nil {
					return nil, err
				}

				sshArgs := sshutil.SSHArgsFromOpts(sshOpts)
				sshCmd = fmt.Sprintf("ssh -p %d %s", cp.instance.SSHLocalPort, strings.Join(sshArgs, " "))
			}
		}
	}

	if sshCmd != "" {
		rsyncArgs = append(rsyncArgs, "-e", sshCmd)
	}

	for _, cp := range copyPaths {
		if cp.isRemote {
			rsyncArgs = append(rsyncArgs, fmt.Sprintf("%s@127.0.0.1:%s", *cp.instance.Config.User.Name, cp.path))
		} else {
			rsyncArgs = append(rsyncArgs, cp.path)
		}
	}

	return exec.CommandContext(ctx, command, rsyncArgs...), nil
}
