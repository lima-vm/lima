// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"

	"al.essio.dev/pkg/shellescape"
	"github.com/coreos/go-semver/semver"
	"github.com/lima-vm/sshocker/pkg/ssh"
	"github.com/mattn/go-isatty"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/lima-vm/lima/v2/pkg/autostart"
	"github.com/lima-vm/lima/v2/pkg/envutil"
	"github.com/lima-vm/lima/v2/pkg/instance"
	"github.com/lima-vm/lima/v2/pkg/ioutilx"
	"github.com/lima-vm/lima/v2/pkg/limatype"
	"github.com/lima-vm/lima/v2/pkg/networks/reconcile"
	"github.com/lima-vm/lima/v2/pkg/sshutil"
	"github.com/lima-vm/lima/v2/pkg/store"
)

const shellHelp = `Execute shell in Lima

lima command is provided as an alias for limactl shell $LIMA_INSTANCE. $LIMA_INSTANCE defaults to "` + DefaultInstanceName + `".

By default, the first 'ssh' executable found in the host's PATH is used to connect to the Lima instance.
A custom ssh alias can be used instead by setting the $` + sshutil.EnvShellSSH + ` environment variable.

Environment Variables:
  --preserve-env: Propagates host environment variables to the guest instance.
                  Use LIMA_SHELLENV_ALLOW to specify which variables to allow.
                  Use LIMA_SHELLENV_BLOCK to specify which variables to block (extends default blocklist with +).

Hint: try --debug to show the detailed logs, if it seems hanging (mostly due to some SSH issue).
`

func newShellCommand() *cobra.Command {
	shellCmd := &cobra.Command{
		Use:               "shell [flags] INSTANCE [COMMAND...]",
		SuggestFor:        []string{"ssh"},
		Short:             "Execute shell in Lima",
		Long:              shellHelp,
		Args:              WrapArgsError(cobra.MinimumNArgs(1)),
		RunE:              shellAction,
		ValidArgsFunction: shellBashComplete,
		SilenceErrors:     true,
		GroupID:           basicCommand,
	}

	shellCmd.Flags().SetInterspersed(false)

	shellCmd.Flags().String("shell", "", "Shell interpreter, e.g. /bin/bash")
	shellCmd.Flags().String("workdir", "", "Working directory")
	shellCmd.Flags().Bool("reconnect", false, "Reconnect to the SSH session")
	shellCmd.Flags().Bool("preserve-env", false, "Propagate environment variables to the shell")
	shellCmd.Flags().Bool("start", false, "Start the instance if it is not already running")
	return shellCmd
}

func shellAction(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	flags := cmd.Flags()
	tty, err := flags.GetBool("tty")
	if err != nil {
		return err
	}
	// simulate the behavior of double dash
	newArg := []string{}
	if len(args) >= 2 && args[1] == "--" {
		newArg = append(newArg, args[:1]...)
		newArg = append(newArg, args[2:]...)
		args = newArg
	}
	instName := args[0]

	if len(args) >= 2 {
		switch args[1] {
		case "create", "start", "delete", "shell":
			// `lima start` (alias of `limactl $LIMA_INSTANCE start`) is probably a typo of `limactl start`
			logrus.Warnf("Perhaps you meant `limactl %s`?", strings.Join(args[1:], " "))
		}
	}

	inst, err := store.Inspect(ctx, instName)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("instance %q does not exist, run `limactl create %s` to create a new instance", instName, instName)
		}
		return err
	}
	if inst.Config == nil {
		if len(inst.Errors) > 0 {
			return fmt.Errorf("instance %q has configuration errors: %w", instName, errors.Join(inst.Errors...))
		}
		return fmt.Errorf("instance %q has no configuration", instName)
	}
	if inst.Status == limatype.StatusStopped {
		startNow, err := flags.GetBool("start")
		if err != nil {
			return err
		}

		if tty && !flags.Changed("start") {
			startNow, err = askWhetherToStart(cmd)
			if err != nil {
				return err
			}
		}

		if !startNow {
			return nil
		}

		// Network reconciliation will be performed by the process launched by the autostart manager
		if registered, err := autostart.IsRegistered(ctx, inst); err != nil && !errors.Is(err, autostart.ErrNotSupported) {
			return fmt.Errorf("failed to check if the autostart entry for instance %q is registered: %w", inst.Name, err)
		} else if !registered {
			err = reconcile.Reconcile(ctx, inst.Name)
			if err != nil {
				return err
			}
		}

		err = instance.Start(ctx, inst, false, false)
		if err != nil {
			return err
		}

		inst, err = store.Inspect(ctx, instName)
		if err != nil {
			return err
		}
	}

	restart, err := cmd.Flags().GetBool("reconnect")
	if err != nil {
		return err
	}
	if restart && sshutil.IsControlMasterExisting(inst.Dir) {
		logrus.Infof("Exiting ssh session for the instance %q", instName)

		sshConfig := &ssh.SSHConfig{
			ConfigFile:     inst.SSHConfigFile,
			Persist:        false,
			AdditionalArgs: []string{},
		}

		if err := ssh.ExitMaster(inst.Hostname, inst.SSHLocalPort, sshConfig); err != nil {
			return err
		}
	}

	// When workDir is explicitly set, the shell MUST have workDir as the cwd, or exit with an error.
	//
	// changeDirCmd := "cd workDir || exit 1"                  if workDir != ""
	//              := "cd hostCurrentDir || cd hostHomeDir"   if workDir == ""
	var changeDirCmd string
	workDir, err := cmd.Flags().GetString("workdir")
	if err != nil {
		return err
	}
	if workDir != "" {
		changeDirCmd = fmt.Sprintf("cd %s || exit 1", shellescape.Quote(workDir))
		// FIXME: check whether y.Mounts contains the home, not just len > 0
	} else if len(inst.Config.Mounts) > 0 || inst.VMType == limatype.WSL2 {
		hostCurrentDir, err := os.Getwd()
		if err == nil && runtime.GOOS == "windows" {
			hostCurrentDir, err = mountDirFromWindowsDir(ctx, inst, hostCurrentDir)
		}
		if err == nil {
			changeDirCmd = fmt.Sprintf("cd %s", shellescape.Quote(hostCurrentDir))
		} else {
			changeDirCmd = "false"
			logrus.WithError(err).Warn("failed to get the current directory")
		}
		hostHomeDir, err := os.UserHomeDir()
		if err == nil && runtime.GOOS == "windows" {
			hostHomeDir, err = mountDirFromWindowsDir(ctx, inst, hostHomeDir)
		}
		if err == nil {
			changeDirCmd = fmt.Sprintf("%s || cd %s", changeDirCmd, shellescape.Quote(hostHomeDir))
		} else {
			logrus.WithError(err).Warn("failed to get the home directory")
		}
	} else {
		logrus.Debug("the host home does not seem mounted, so the guest shell will have a different cwd")
	}

	if changeDirCmd == "" {
		changeDirCmd = "false"
	}
	logrus.Debugf("changeDirCmd=%q", changeDirCmd)

	shell, err := cmd.Flags().GetString("shell")
	if err != nil {
		return err
	}
	if shell == "" {
		shell = `"$SHELL"`
	} else {
		shell = shellescape.Quote(shell)
	}
	// Handle environment variable propagation
	var envPrefix string
	preserveEnv, err := cmd.Flags().GetBool("preserve-env")
	if err != nil {
		return err
	}
	if preserveEnv {
		filteredEnv := envutil.FilterEnvironment()
		if len(filteredEnv) > 0 {
			envPrefix = "env "
			for _, envVar := range filteredEnv {
				envPrefix += shellescape.Quote(envVar) + " "
			}
		}
	}

	script := fmt.Sprintf("%s ; exec %s%s --login", changeDirCmd, envPrefix, shell)
	if len(args) > 1 {
		quotedArgs := make([]string, len(args[1:]))
		parsingEnv := true
		for i, arg := range args[1:] {
			if parsingEnv && isEnv(arg) {
				quotedArgs[i] = quoteEnv(arg)
			} else {
				parsingEnv = false
				quotedArgs[i] = shellescape.Quote(arg)
			}
		}
		script += fmt.Sprintf(
			" -c %s",
			shellescape.Quote(strings.Join(quotedArgs, " ")),
		)
	}

	sshExe, err := sshutil.NewSSHExe()
	if err != nil {
		return err
	}

	sshOpts, err := sshutil.SSHOpts(
		ctx,
		sshExe,
		inst.Dir,
		*inst.Config.User.Name,
		*inst.Config.SSH.LoadDotSSHPubKeys,
		*inst.Config.SSH.ForwardAgent,
		*inst.Config.SSH.ForwardX11,
		*inst.Config.SSH.ForwardX11Trusted)
	if err != nil {
		return err
	}
	if runtime.GOOS == "windows" {
		// Remove ControlMaster, ControlPath, and ControlPersist options,
		// because Cygwin-based SSH clients do not support multiplexing when executing commands.
		// References:
		//   https://inbox.sourceware.org/cygwin/c98988a5-7e65-4282-b2a1-bb8e350d5fab@acm.org/T/
		//   https://stackoverflow.com/questions/20959792/is-ssh-controlmaster-with-cygwin-on-windows-actually-possible
		// By removing these options:
		//   - Avoids execution failures when the control master is not yet available.
		//   - Prevents error messages such as:
		//     > mux_client_request_session: read from master failed: Connection reset by peer
		//     > ControlSocket ....sock already exists, disabling multiplexing
		// Only remove these options when writing the SSH config file and executing `limactl shell`, since multiplexing seems to work with port forwarding.
		sshOpts = sshutil.SSHOptsRemovingControlPath(sshOpts)
	}
	sshArgs := append([]string{}, sshExe.Args...)
	sshArgs = append(sshArgs, sshutil.SSHArgsFromOpts(sshOpts)...)
	if isatty.IsTerminal(os.Stdout.Fd()) || isatty.IsCygwinTerminal(os.Stdout.Fd()) {
		// required for showing the shell prompt: https://stackoverflow.com/a/626574
		sshArgs = append(sshArgs, "-t")
	}
	if _, present := os.LookupEnv("COLORTERM"); present {
		// SendEnv config is cumulative, with already existing options in ssh_config
		sshArgs = append(sshArgs, "-o", "SendEnv=COLORTERM")
	}
	logLevel := "ERROR"
	// For versions older than OpenSSH 8.9p, LogLevel=QUIET was needed to
	// avoid the "Shared connection to 127.0.0.1 closed." message with -t.
	olderSSH := sshutil.DetectOpenSSHVersion(ctx, sshExe).LessThan(*semver.New("8.9.0"))
	if olderSSH {
		logLevel = "QUIET"
	}
	sshArgs = append(sshArgs, []string{
		"-o", fmt.Sprintf("LogLevel=%s", logLevel),
		"-p", strconv.Itoa(inst.SSHLocalPort),
		inst.SSHAddress,
		"--",
		script,
	}...)
	sshCmd := exec.CommandContext(ctx, sshExe.Exe, sshArgs...)
	sshCmd.Stdin = os.Stdin
	sshCmd.Stdout = os.Stdout
	sshCmd.Stderr = os.Stderr
	logrus.Debugf("executing ssh (may take a long)): %+v", sshCmd.Args)

	// TODO: use syscall.Exec directly (results in losing tty?)
	return sshCmd.Run()
}

func mountDirFromWindowsDir(ctx context.Context, inst *limatype.Instance, dir string) (string, error) {
	if inst.VMType == limatype.WSL2 {
		distroName := "lima-" + inst.Name
		return ioutilx.WindowsSubsystemPathForLinux(ctx, dir, distroName)
	}
	return ioutilx.WindowsSubsystemPath(ctx, dir)
}

func shellBashComplete(cmd *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	return bashCompleteInstanceNames(cmd)
}

func isEnv(arg string) bool {
	return len(strings.Split(arg, "=")) > 1
}

func quoteEnv(arg string) string {
	env := strings.SplitN(arg, "=", 2)
	env[1] = shellescape.Quote(env[1])
	return strings.Join(env, "=")
}
