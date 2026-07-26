// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package copytool

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"al.essio.dev/pkg/shellescape"
	"github.com/sirupsen/logrus"

	"github.com/lima-vm/lima/v2/pkg/limatype"
	"github.com/lima-vm/lima/v2/pkg/sshutil"
)

type rsyncTool struct {
	toolPath string
	Options  *Options
}

func newRsyncTool(opts *Options) (*rsyncTool, error) {
	toolPath, err := exec.LookPath("rsync")
	if err != nil {
		return nil, fmt.Errorf("rsync not found on host: %w", err)
	}
	return &rsyncTool{toolPath: toolPath, Options: opts}, nil
}

func (t *rsyncTool) Name() string {
	return t.toolPath
}

func (t *rsyncTool) IsAvailableOnGuest(ctx context.Context, paths []string) bool {
	copyPaths, err := parseCopyPaths(ctx, paths)
	if err != nil {
		// New() has already reported this to the user.
		logrus.Debugf("failed to parse copy paths for rsync availability check: %v", err)
		return false
	}
	instances := make(map[string]*limatype.Instance)

	for _, cp := range copyPaths {
		if cp.IsRemote {
			instances[cp.InstanceName] = cp.Instance
		}
	}

	for instName, inst := range instances {
		if !checkRsyncOnGuest(ctx, inst) {
			logrus.Debugf("rsync not available on instance %#q", instName)
			return false
		}
	}

	return true
}

func checkRsyncOnGuest(ctx context.Context, inst *limatype.Instance) bool {
	checkCmd, err := sshCommandOnGuest(ctx, inst, "command -v rsync >/dev/null 2>&1")
	if err != nil {
		logrus.Debugf("failed to prepare the SSH command for the rsync check: %v", err)
		return false
	}
	return checkCmd.Run() == nil
}

// sshCommandOnGuest returns a command that executes script with the shell of the guest.
func sshCommandOnGuest(ctx context.Context, inst *limatype.Instance, script string) (*exec.Cmd, error) {
	sshExe, err := sshutil.NewSSHExe()
	if err != nil {
		return nil, err
	}
	sshOpts, err := sshutil.SSHOpts(ctx, sshExe, inst.Dir, *inst.Config.User.Name, false, false, false, false)
	if err != nil {
		return nil, err
	}

	sshArgs := append([]string{}, sshExe.Args...)
	sshArgs = append(sshArgs, sshutil.SSHArgsFromOpts(sshOpts)...)
	sshArgs = append(sshArgs,
		"-p", fmt.Sprintf("%d", inst.SSHLocalPort),
		*inst.Config.User.Name+"@"+inst.SSHAddress,
		script,
	)
	return exec.CommandContext(ctx, sshExe.Exe, sshArgs...), nil
}

func (t *rsyncTool) Command(ctx context.Context, paths []string, opts *Options) (*exec.Cmd, error) {
	copyPaths, err := parseCopyPaths(ctx, paths)
	if err != nil {
		return nil, err
	}

	effectiveOpts := t.Options
	if opts != nil {
		effectiveOpts = opts
	}

	rsyncFlags := []string{"-a"}

	if effectiveOpts.Verbose {
		rsyncFlags = append(rsyncFlags, "-v", "--progress")
	} else {
		rsyncFlags = append(rsyncFlags, "-q")
	}

	if effectiveOpts.Recursive {
		rsyncFlags = append(rsyncFlags, "-r")
	}

	if effectiveOpts.AdditionalArgs != nil {
		rsyncFlags = append(rsyncFlags, effectiveOpts.AdditionalArgs...)
	}

	rsyncArgs := make([]string, 0, len(rsyncFlags)+len(copyPaths))
	rsyncArgs = append(rsyncArgs, rsyncFlags...)

	var sshCmd string
	var remoteInstance *limatype.Instance

	for _, cp := range copyPaths {
		if cp.IsRemote {
			if remoteInstance == nil {
				remoteInstance = cp.Instance
				sshExe, err := sshutil.NewSSHExe()
				if err != nil {
					return nil, err
				}
				sshOpts, err := sshutil.SSHOpts(ctx, sshExe, cp.Instance.Dir, *cp.Instance.Config.User.Name, false, false, false, false)
				if err != nil {
					return nil, err
				}

				sshArgs := []string{sshExe.Exe}
				sshArgs = append(sshArgs, sshExe.Args...)
				sshArgs = append(sshArgs, sshutil.SSHArgsFromOpts(sshOpts)...)
				sshArgs = append(sshArgs, "-p", fmt.Sprintf("%d", cp.Instance.SSHLocalPort))

				quotedSSHArgs := make([]string, len(sshArgs))
				for i, arg := range sshArgs {
					quotedSSHArgs[i] = shellescape.Quote(arg)
				}
				sshCmd = strings.Join(quotedSSHArgs, " ")
			}
		}
	}

	if sshCmd != "" {
		rsyncArgs = append(rsyncArgs, "-e", sshCmd)
	}

	// Adjust the trailing slashes of the paths so that a recursive copy behaves like
	// `cp -r` and `scp -r`, which was the original implementation of `limactl copy -r`.
	// https://github.com/lima-vm/lima/issues/4468
	// https://github.com/lima-vm/lima/issues/5500
	if effectiveOpts.Recursive && len(copyPaths) > 1 {
		if err := adjustTrailingSlashes(ctx, paths, copyPaths); err != nil {
			return nil, err
		}
	}

	// End option parsing so a path starting with a dash is treated as a path,
	// not as an rsync option. scp.go does the same.
	rsyncArgs = append(rsyncArgs, "--")

	for _, cp := range copyPaths {
		if cp.IsRemote {
			rsyncArgs = append(rsyncArgs, fmt.Sprintf("%s:%s", *cp.Instance.Config.User.Name+"@"+cp.Instance.SSHAddress, cp.Path))
		} else {
			rsyncArgs = append(rsyncArgs, cp.Path)
		}
	}

	return exec.CommandContext(ctx, t.toolPath, rsyncArgs...), nil
}

// adjustTrailingSlashes rewrites the source paths of a recursive copy so that rsync
// behaves like `cp -r` and `scp -r`: the source directory is copied into the destination
// when the destination is an existing directory (DST/SRC), and becomes the destination
// when it does not exist yet. rsync decides this from a trailing slash on the source
// instead: "SRC/" copies the *contents* of SRC, while "SRC" always creates DST/SRC.
//
// paths are the original command line arguments, corresponding to copyPaths by index.
// The last one is the destination; the preceding ones are the sources.
func adjustTrailingSlashes(ctx context.Context, paths []string, copyPaths []*Path) error {
	last := len(copyPaths) - 1
	srcs, dst := copyPaths[:last], copyPaths[last]

	// A trailing slash on a source is not significant for `limactl copy`.
	for _, src := range srcs {
		src.Path = trimTrailingSlashes(src.Path)
	}

	// Multiple sources always require the destination to be a directory.
	if len(srcs) != 1 {
		return nil
	}
	// Otherwise rsync has to be told to copy the contents of the source when the
	// destination is not an existing directory, so that the destination becomes a copy of
	// the source directory rather than its parent.
	dstIsDir, err := isDir(ctx, paths[last], dst)
	if err != nil || dstIsDir {
		return err
	}
	// A source that is not a directory is already copied by name, and a trailing slash
	// would just make rsync fail on it.
	srcIsDir, err := isDir(ctx, paths[0], srcs[0])
	if err != nil {
		return err
	}
	if srcIsDir {
		srcs[0].Path += "/"
	}
	return nil
}

// trimTrailingSlashes removes the trailing slashes from path, unless the path is made of
// slashes only (e.g. "/"), as trimming those would turn it into an empty path.
func trimTrailingSlashes(path string) string {
	if trimmed := strings.TrimRight(path, "/"); trimmed != "" {
		return trimmed
	}
	return path
}

// isDir reports whether the path of cp is an existing directory. A path that does not
// exist is not an error, but failing to determine the answer is: guessing it would copy
// the files to the wrong place.
//
// rawPath is the corresponding command line argument. It is used instead of cp.Path for
// local paths, because cp.Path may have been rewritten to a subsystem path (e.g.
// "/cygdrive/c/...") that only the SSH executable understands on Windows hosts.
func isDir(ctx context.Context, rawPath string, cp *Path) (bool, error) {
	if !cp.IsRemote {
		st, err := os.Stat(rawPath)
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		return st.IsDir(), nil
	}
	path := cp.Path
	if path == "" {
		// `limactl copy foo inst:` copies into the login directory of the guest.
		path = "."
	}
	cmd, err := sshCommandOnGuest(ctx, cp.Instance, "test -d "+quoteRemotePath(path))
	if err != nil {
		return false, err
	}
	// `test` writes nothing, so Output() only serves to capture the stderr of SSH.
	if _, err = cmd.Output(); err == nil {
		return true, nil
	}
	// `test` exits with 1 when the path is not a directory. Any other status means that
	// the answer is unknown; SSH itself exits with 255 when it fails to connect.
	if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
		if exitErr.ExitCode() == 1 {
			return false, nil
		}
		if stderr := strings.TrimSpace(string(exitErr.Stderr)); stderr != "" {
			err = fmt.Errorf("%w (%s)", err, stderr)
		}
	}
	return false, fmt.Errorf("failed to check whether %#q is a directory on instance %#q: %w",
		path, cp.InstanceName, err)
}

// tildePrefixRegexp matches a "~" or "~user" prefix that the shell of the guest expands.
var tildePrefixRegexp = regexp.MustCompile(`^~[A-Za-z0-9._-]*$`)

// quoteRemotePath quotes path for the shell of the guest, keeping a leading "~" or
// "~user" unquoted so that the shell still expands it, like it does for the paths that
// rsync passes to the guest.
func quoteRemotePath(path string) string {
	prefix, rest, found := strings.Cut(path, "/")
	if !tildePrefixRegexp.MatchString(prefix) {
		return shellescape.Quote(path)
	}
	if !found {
		return prefix
	}
	return prefix + "/" + shellescape.Quote(rest)
}
