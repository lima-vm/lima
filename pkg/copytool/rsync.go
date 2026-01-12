// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package copytool

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

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
		*inst.Config.User.Name+"@"+inst.SSHAddress,
		"command -v rsync >/dev/null 2>&1",
	)

	err = checkCmd.Run()
	return err == nil
}

func (t *rsyncTool) Command(ctx context.Context, paths []string, opts *Options) (*exec.Cmd, error) {
	copyPaths, err := parseCopyPaths(ctx, paths)
	if err != nil {
		return nil, err
	}

	if opts != nil {
		t.Options = opts
	}

	rsyncFlags := []string{"-a"}

	if t.Options.Verbose {
		rsyncFlags = append(rsyncFlags, "-v", "--progress")
	} else {
		rsyncFlags = append(rsyncFlags, "-q")
	}

	if t.Options.Recursive {
		rsyncFlags = append(rsyncFlags, "-r")
	}

	if t.Options.AdditionalArgs != nil {
		rsyncFlags = append(rsyncFlags, t.Options.AdditionalArgs...)
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

				sshArgs := sshutil.SSHArgsFromOpts(sshOpts)
				sshCmd = fmt.Sprintf("ssh -p %d %s", cp.Instance.SSHLocalPort, strings.Join(sshArgs, " "))
			}
		}
	}

	if sshCmd != "" {
		rsyncArgs = append(rsyncArgs, "-e", sshCmd)
	}

	// Handle trailing slash for directory copies to keep consistent behavior with scp,
	// which was the original implementation of `limactl copy -r`.
	// https://github.com/lima-vm/lima/issues/4468
	if t.Options.Recursive {
		for i, cp := range copyPaths {
			//nolint:modernize // stringscutprefix: HasSuffix + TrimSuffix can be simplified to CutSuffix
			if strings.HasSuffix(cp.Path, "/") {
				if cp.IsRemote {
					for j, cp2 := range copyPaths {
						if i != j {
							cp2.Path = strings.TrimSuffix(cp2.Path, "/")
						}
					}
				} else {
					cp.Path = strings.TrimSuffix(cp.Path, "/")
				}
			} else {
				cp.Path += "/"
			}
		}
	}

	for _, cp := range copyPaths {
		if cp.IsRemote {
			rsyncArgs = append(rsyncArgs, fmt.Sprintf("%s:%s", *cp.Instance.Config.User.Name+"@"+cp.Instance.SSHAddress, cp.Path))
		} else {
			rsyncArgs = append(rsyncArgs, cp.Path)
		}
	}

	return exec.CommandContext(ctx, t.toolPath, rsyncArgs...), nil
}
