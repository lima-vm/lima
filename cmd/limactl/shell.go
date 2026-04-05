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
	"regexp"
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
	"github.com/lima-vm/lima/v2/pkg/copytool"
	"github.com/lima-vm/lima/v2/pkg/envutil"
	"github.com/lima-vm/lima/v2/pkg/instance"
	"github.com/lima-vm/lima/v2/pkg/ioutilx"
	"github.com/lima-vm/lima/v2/pkg/limatype"
	"github.com/lima-vm/lima/v2/pkg/localpathutil"
	"github.com/lima-vm/lima/v2/pkg/networks/reconcile"
	"github.com/lima-vm/lima/v2/pkg/sshutil"
	"github.com/lima-vm/lima/v2/pkg/store"
	"github.com/lima-vm/lima/v2/pkg/uiutil"
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
		Args:              WrapArgsError(cobra.ArbitraryArgs),
		RunE:              shellAction,
		ValidArgsFunction: shellBashComplete,
		SilenceErrors:     true,
		GroupID:           basicCommand,
	}

	shellCmd.Flags().SetInterspersed(false)

	shellCmd.Flags().String("instance", "", "Instance name (used by the lima wrapper script)")
	_ = shellCmd.Flags().MarkHidden("instance")
	shellCmd.Flags().String("shell", "", "Shell interpreter, e.g. /bin/bash")
	shellCmd.Flags().String("workdir", "", "Working directory")
	shellCmd.Flags().Bool("reconnect", false, "Reconnect to the SSH session")
	shellCmd.Flags().Bool("preserve-env", false, "Propagate environment variables to the shell")
	shellCmd.Flags().Bool("start", false, "Start the instance if it is not already running")
	shellCmd.Flags().String("sync", "", "Copy a host directory to the guest and vice-versa upon exit")

	return shellCmd
}

const (
	rsyncMinimumSrcDirDepth = 4 // Depth of "/Users/USER" is 3.
	colorGray               = "\033[0;90m"
	colorNone               = "\033[0m"
)

func shellAction(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	flags := cmd.Flags()
	tty, err := flags.GetBool("tty")
	if err != nil {
		return err
	}

	// When --instance is specified, all positional args are treated as COMMAND.
	// Otherwise, the first positional arg is the instance name (backward compatible).
	instName, err := flags.GetString("instance")
	if err != nil {
		return err
	}
	if instName != "" {
		// All args are COMMAND; prepend a placeholder instance name so the rest of the code works unchanged.
		args = append([]string{instName}, args...)
	} else {
		if len(args) == 0 {
			return errors.New("requires instance name as first argument")
		}
		// simulate the behavior of double dash
		newArg := []string{}
		if len(args) >= 2 && args[1] == "--" {
			newArg = append(newArg, args[:1]...)
			newArg = append(newArg, args[2:]...)
			args = newArg
		}
		instName = args[0]
	}

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
	if len(inst.Errors) > 0 {
		logrus.WithError(errors.Join(inst.Errors...)).Errorf("Instance %q has configuration errors", instName)
	}
	if inst.Config == nil {
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
			return fmt.Errorf("instance %q is stopped, run `limactl start %s` to start the instance", instName, instName)
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

		err = instance.Start(instance.WithLaunchingShell(ctx), inst, false, false)
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

	syncDirVal, err := flags.GetString("sync")
	if err != nil {
		return fmt.Errorf("failed to get sync flag: %w", err)
	}
	syncHostWorkdir := syncDirVal != ""
	if syncHostWorkdir && len(inst.Config.Mounts) > 0 {
		return errors.New("cannot use `--sync` when the instance has host mounts configured, start the instance with `--mount-none` to disable mounts")
	}

	// When workDir is explicitly set, the shell MUST have workDir as the cwd, or exit with an error.
	//
	// changeDirCmd := "cd workDir || exit 1"                  if workDir != ""
	//              := "cd hostCurrentDir || cd hostHomeDir"   if workDir == ""
	var changeDirCmd string
	var hostCurrentDir string
	if syncDirVal != "" {
		hostCurrentDir, err = filepath.Abs(syncDirVal)
		if err == nil && runtime.GOOS == "windows" {
			hostCurrentDir, err = mountDirFromWindowsDir(ctx, inst, hostCurrentDir)
		}
	} else {
		hostCurrentDir, err = hostCurrentDirectory(ctx, inst)
	}

	if err != nil {
		changeDirCmd = "false"
		logrus.WithError(err).Warn("failed to get the current directory")
	}
	if syncHostWorkdir {
		if _, err := exec.LookPath(string(copytool.BackendRsync)); err != nil {
			return fmt.Errorf("rsync is required for `--sync` but not found: %w", err)
		}

		srcWdDepth := len(strings.Split(hostCurrentDir, string(os.PathSeparator)))
		if srcWdDepth < rsyncMinimumSrcDirDepth {
			return fmt.Errorf("expected the depth of the host working directory (%q) to be more than %d, only got %d (Hint: %s)",
				hostCurrentDir, rsyncMinimumSrcDirDepth, srcWdDepth, "cd to a deeper directory")
		}
	}

	var destRsyncDir string
	workDir, err := cmd.Flags().GetString("workdir")
	if err != nil {
		return err
	}
	if workDir != "" && syncHostWorkdir {
		return errors.New("cannot use `--workdir` and `--sync` at the same time")
	}
	if syncHostWorkdir {
		destRsyncDir = *inst.Config.User.Home + hostCurrentDir
	}
	hostHomeDir, err := os.UserHomeDir()
	if err == nil && runtime.GOOS == "windows" {
		hostHomeDir, err = mountDirFromWindowsDir(ctx, inst, hostHomeDir)
	}
	if err != nil {
		logrus.WithError(err).Warn("failed to get the home directory")
		hostHomeDir = ""
	}

	switch {
	case workDir != "":
		changeDirCmd = fmt.Sprintf("cd %s || exit 1", shellescape.Quote(workDir))
	case mountsContainPath(inst.Config.Mounts, hostCurrentDir) || inst.VMType == limatype.WSL2:
		changeDirCmd = fmt.Sprintf("cd %s", shellescape.Quote(hostCurrentDir))
		if hostHomeDir != "" {
			changeDirCmd = fmt.Sprintf("%s || cd %s", changeDirCmd, shellescape.Quote(hostHomeDir))
		}
	case syncHostWorkdir:
		changeDirCmd = fmt.Sprintf("cd %s", shellescape.Quote(destRsyncDir))
	case hostHomeDir != "" && mountsContainPath(inst.Config.Mounts, hostHomeDir):
		changeDirCmd = fmt.Sprintf("cd %s", shellescape.Quote(hostHomeDir))
	default:
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

	// -l is known to be available in bash, zsh, and FreeBSD sh.
	// Note that --login is not available in FreeBSD sh.
	script := fmt.Sprintf("%s ; exec %s%s -l", changeDirCmd, envPrefix, shell)
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

	var (
		sshExecForRsync *exec.Cmd
		rsync           copytool.CopyTool
	)
	if syncHostWorkdir {
		logrus.Infof("Syncing host current directory(%s) to guest instance...", hostCurrentDir)
		sshExecForRsync = exec.CommandContext(ctx, sshExe.Exe, sshArgs...)

		// Create the destination directory in the guest instance,
		// we could have done this by using `--rsync-path` but it's more
		// complex to quote properly.
		if err := executeSSHForRsync(ctx, *sshExecForRsync, inst.SSHLocalPort, inst.SSHAddress, fmt.Sprintf("mkdir -p %s", shellescape.Quote(destRsyncDir))); err != nil {
			return fmt.Errorf("failed to create the synced workdir in guest instance: %w", err)
		}

		// The macOS release of rsync (the latest being 2.6.9) does not support shell escaping of destination path but other versions do.
		rsyncVer, err := rsyncVersion(ctx)
		if err != nil {
			return fmt.Errorf("failed to get rsync version: %w", err)
		}
		if rsyncVer.LessThan(*semver.New("3.0.0")) {
			destRsyncDir = shellescape.Quote(destRsyncDir)
		}

		paths := []string{
			hostCurrentDir,
			fmt.Sprintf("%s:%s", inst.Name, destRsyncDir),
		}
		rsync, err = copytool.New(ctx, string(copytool.BackendRsync), paths, &copytool.Options{
			Recursive: true,
			Verbose:   false,
			AdditionalArgs: []string{
				"--delete",
			},
		})
		if err != nil {
			return err
		}
		logrus.Debugf("using copy tool %q", rsync.Name())

		if err := rsyncDirectory(ctx, cmd, rsync, paths); err != nil {
			return fmt.Errorf("failed to rsync to the guest %w", err)
		}
		logrus.Infof("Successfully synced host current directory to guest(%s) instance.", destRsyncDir)
	}

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
	if err := sshCmd.Run(); err != nil {
		return err
	}

	// Once the shell command finishes, rsync back the changes from guest workdir
	// to the host and delete the guest synced workdir only if the user
	// confirms the changes.
	if syncHostWorkdir {
		tty, err := flags.GetBool("tty")
		if err != nil {
			return err
		}
		return askUserForRsyncBack(ctx, cmd, inst, sshExecForRsync, hostCurrentDir, destRsyncDir, rsync, tty)
	}
	return nil
}

func askUserForRsyncBack(ctx context.Context, cmd *cobra.Command, inst *limatype.Instance, sshCmd *exec.Cmd, hostCurrentDir, destRsyncDir string, rsync copytool.CopyTool, tty bool) error {
	remoteSource := fmt.Sprintf("%s:%s", inst.Name, destRsyncDir)
	clean := filepath.Clean(hostCurrentDir)
	dirForCleanup := shellescape.Quote(filepath.Join(*inst.Config.User.Home, clean))

	rsyncBack := func() error {
		paths := []string{
			remoteSource,
			hostCurrentDir,
		}

		if err := rsyncDirectory(ctx, cmd, rsync, paths); err != nil {
			return fmt.Errorf("failed to sync back the changes from guest instance to host: %w", err)
		}
		logrus.Info("Successfully synced back the changes to host.")
		return nil
	}

	defer func() {
		// Clean up the guest synced workdir
		if err := executeSSHForRsync(ctx, *sshCmd, inst.SSHLocalPort, inst.SSHAddress, fmt.Sprintf("rm -rf %s", dirForCleanup)); err != nil {
			logrus.WithError(err).Warn("Failed to clean up guest synced workdir")
		}
	}()

	if !tty {
		return rsyncBack()
	}

	rawOutput, stats, err := getRsyncStats(ctx, remoteSource, filepath.Dir(hostCurrentDir))
	if err != nil {
		logrus.WithError(err).Warn("failed to get rsync stats")
	}
	if stats != nil && stats.String() == "" {
		logrus.Info("No changes detected")
		return nil
	}
	statsMsg := ""
	if stats != nil {
		if s := stats.String(); s != "" {
			statsMsg = fmt.Sprintf(" (%s)", s)
		}
	}

	message := fmt.Sprintf("⚠️ Accept the changes?%s", statsMsg)
	options := []string{
		"Yes",
		"No",
		"View the changed contents",
	}

	baseDir, err := os.MkdirTemp("", "lima-guest-synced-*")
	if err != nil {
		return err
	}
	defer func() {
		if err := os.RemoveAll(baseDir); err != nil {
			logrus.WithError(err).Warnf("Failed to clean up temporary directory %s", baseDir)
		}
	}()
	hostTmpDest := filepath.Join(baseDir, filepath.Base(hostCurrentDir))
	err = os.MkdirAll(hostTmpDest, 0o755)
	if err != nil {
		return err
	}

	rsyncToTempDir := false

	for {
		ans, err := uiutil.Select(message, options)
		if err != nil {
			return fmt.Errorf("failed to open TUI: %w", err)
		}

		switch ans {
		case 0: // Yes
			return rsyncBack()
		case 1: // No
			logrus.Info("Skipping syncing back the changes to host.")
			return nil
		case 2: // View the changed contents
			var diffCmd *exec.Cmd
			if _, err := exec.LookPath("diff"); err != nil {
				logrus.WithError(err).Warn("`diff` not found; showing rsync dry-run output only")
			} else {
				diffCmd = exec.CommandContext(ctx, "diff", "-ruN", "--color=always", hostCurrentDir, hostTmpDest)
				if !rsyncToTempDir {
					paths := []string{
						remoteSource,
						hostTmpDest,
					}

					if err := rsyncDirectory(ctx, cmd, rsync, paths); err != nil {
						return fmt.Errorf("failed to sync back the changes from guest instance to host temporary directory: %w", err)
					}
					rsyncToTempDir = true
				}
			}

			pagerArgs := pagerCommand()
			lessCmd := exec.CommandContext(ctx, pagerArgs[0], pagerArgs[1:]...)

			pipeIn, err := lessCmd.StdinPipe()
			if err != nil {
				return fmt.Errorf("failed to create pipe for less: %w", err)
			}
			if diffCmd != nil {
				diffCmd.Stdout = pipeIn
			}
			lessCmd.Stdout = cmd.OutOrStdout()
			lessCmd.Stderr = cmd.OutOrStderr()

			if err := lessCmd.Start(); err != nil {
				return fmt.Errorf("failed to start less: %w", err)
			}

			// Write rsync dry-run output first
			if stats != nil {
				rsyncHead := fmt.Sprintf("%s--- rsync dry-run statistics ---%s", colorGray, colorNone)
				diffHead := fmt.Sprintf("%s--- detailed diff --- %s", colorGray, colorNone)
				if diffCmd == nil {
					diffHead = fmt.Sprintf("%s--- detailed diff unavailable (`diff` not found) --- %s", colorGray, colorNone)
				}
				combinedOutput := fmt.Sprintf(
					"%s\n%s\n\n%s\n\n\n%s\n",
					rsyncHead,
					stats.String(),
					rawOutput,
					diffHead,
				)

				if _, err := fmt.Fprint(pipeIn, combinedOutput); err != nil {
					_ = pipeIn.Close()
					return fmt.Errorf("failed to write rsync stats to pager: %w", err)
				}
			}

			if diffCmd != nil {
				if err := diffCmd.Run(); err != nil {
					// Command `diff` returns exit code 1 when files differ.
					var exitErr *exec.ExitError
					if errors.As(err, &exitErr) && exitErr.ExitCode() >= 2 {
						_ = pipeIn.Close()
						return fmt.Errorf("failed to run diff command: %w", err)
					}
				}
			}

			_ = pipeIn.Close()

			if err := lessCmd.Wait(); err != nil {
				return fmt.Errorf("failed to wait for less command: %w", err)
			}
		}
	}
}

// pagerCommand returns the pager command and arguments to use for viewing colored diff output.
func pagerCommand() []string {
	pager := strings.TrimSpace(os.Getenv("PAGER"))
	if pager == "" {
		return []string{"less", "-R"}
	}
	args := strings.Fields(pager)
	if filepath.Base(args[0]) != "less" {
		return args
	}
	if lessHasRawColorFlag(args[1:]) {
		return args
	}
	if lessHasRawColorFlag(strings.Fields(os.Getenv("LESS"))) {
		return args
	}
	return append(args, "-R")
}

func lessHasRawColorFlag(flags []string) bool {
	for _, f := range flags {
		switch f {
		case "-R", "-r", "--RAW-CONTROL-CHARS", "--raw-control-chars":
			return true
		}
		if strings.HasPrefix(f, "-") && !strings.HasPrefix(f, "--") {
			if strings.ContainsAny(f, "Rr") {
				return true
			}
		}
	}
	return false
}

func mountsContainPath(mounts []limatype.Mount, path string) bool {
	resolvedPath, err := filepath.EvalSymlinks(path)
	if err != nil {
		resolvedPath = path
	}
	for _, m := range mounts {
		loc, err := localpathutil.Expand(m.Location)
		if err != nil {
			continue
		}
		resolvedLoc, err := filepath.EvalSymlinks(loc)
		if err != nil {
			resolvedLoc = loc
		}
		if resolvedPath == resolvedLoc ||
			resolvedLoc == string(os.PathSeparator) || strings.HasPrefix(resolvedPath, resolvedLoc+string(os.PathSeparator)) {
			return true
		}
	}
	return false
}

func executeSSHForRsync(ctx context.Context, sshCmd exec.Cmd, sshLocalPort int, sshAddress, command string) error {
	args := make([]string, len(sshCmd.Args), len(sshCmd.Args)+3)
	copy(args, sshCmd.Args)
	args = append(args, "-p", strconv.Itoa(sshLocalPort), sshAddress)

	// Skip Args[0] (program name) to avoid duplication
	sshRmCmd := exec.CommandContext(ctx, sshCmd.Path, append(args[1:], command)...)
	if err := sshRmCmd.Run(); err != nil {
		return err
	}
	return nil
}

func hostCurrentDirectory(ctx context.Context, inst *limatype.Instance) (string, error) {
	hostCurrentDir, err := os.Getwd()
	if err == nil && runtime.GOOS == "windows" {
		hostCurrentDir, err = mountDirFromWindowsDir(ctx, inst, hostCurrentDir)
	}
	return hostCurrentDir, err
}

func rsyncVersion(ctx context.Context) (*semver.Version, error) {
	out, err := exec.CommandContext(ctx, string(copytool.BackendRsync), "--version").Output()
	if err != nil {
		return nil, err
	}

	// `rsync  version 3.2.7  protocol version 31`
	re := regexp.MustCompile(`version (\d+\.\d+\.\d+)`)
	matches := re.FindSubmatch(out)
	if len(matches) < 2 {
		return nil, errors.New("failed to parse rsync version")
	}
	return semver.NewVersion(string(matches[1]))
}

// Syncs a directory from host to guest and vice-versa. It creates a directory in the guest's home directory and copies the contents of the host's
// current working directory into it. The guest directory paths should be prefixed with `<InstanceName>:` followed by the path.
func rsyncDirectory(ctx context.Context, cmd *cobra.Command, rsync copytool.CopyTool, paths []string) error {
	rsyncCmd, err := rsync.Command(ctx, paths, nil)
	if err != nil {
		return err
	}
	rsyncCmd.Stdout = cmd.OutOrStdout()
	rsyncCmd.Stderr = cmd.OutOrStderr()
	logrus.Debugf("executing rsync: %+v", rsyncCmd.Args)
	return rsyncCmd.Run()
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

type rsyncStats struct {
	Added    int
	Deleted  int
	Modified int
	Metadata int
}

func (s *rsyncStats) String() string {
	if s.Added == 0 && s.Deleted == 0 && s.Modified == 0 && s.Metadata == 0 {
		return ""
	}
	return fmt.Sprintf("added: %d, deleted: %d, modified: %d, metadata: %d", s.Added, s.Deleted, s.Modified, s.Metadata)
}

func getRsyncStats(ctx context.Context, source, destination string) (string, *rsyncStats, error) {
	paths := []string{source, destination}
	rsync, err := copytool.New(ctx, string(copytool.BackendRsync), paths, &copytool.Options{
		Verbose: true,
		AdditionalArgs: []string{
			"--dry-run",
			"--itemize-changes",
			"-ah",
			"--delete",
		},
	})
	if err != nil {
		return "", nil, err
	}

	rsyncCmd, err := rsync.Command(ctx, paths, nil)
	if err != nil {
		return "", nil, err
	}
	logrus.Debugf("executing rsync for stats: %+v", rsyncCmd.Args)

	out, err := rsyncCmd.Output()
	if err != nil {
		return "", nil, err
	}
	output := string(out)
	return output, parseRsyncStats(output), nil
}

// parseRsyncStats parses the output of `rsync --itemize-changes` to extract file operation statistics.
//
// Rsync itemized format: YXcstpoguax  path/to/file
// Where Y=update type, X=file type, c=checksum status, and positions 3-10 are other attributes.
//
// Examples:
//
//	>f+++++++++ file.txt    → Added (new file received)
//	>f.st...... file.txt    → Modified (existing file updated)
//	*deleting   file.txt    → Deleted
//
// Logic:
//
// - `*deleting`: Count as Deleted.
// - Update type `<` (sent), `>` (received), or `c` (local change):
//   - If checksum is `+` (created): Count as Added.
//   - Otherwise: Count as Modified.
//
// - Update type `.` with non-`.` metadata attributes (positions 3-10):
//   - Count as Metadata.
func parseRsyncStats(output string) *rsyncStats {
	var s rsyncStats
	for line := range strings.SplitSeq(output, "\n") {
		if len(line) < 12 {
			continue
		}

		if strings.HasPrefix(line, "*deleting") {
			s.Deleted++
			continue
		}

		updateType := line[0]
		checksum := line[2]

		if updateType == '<' || updateType == '>' || updateType == 'c' {
			if checksum == '+' {
				s.Added++
			} else {
				s.Modified++
			}
			continue
		}

		if updateType == '.' && hasMetadataDelta(line[2:11]) {
			s.Metadata++
		}
	}
	return &s
}

func hasMetadataDelta(attrs string) bool {
	for _, ch := range attrs {
		if ch != '.' {
			return true
		}
	}
	return false
}
