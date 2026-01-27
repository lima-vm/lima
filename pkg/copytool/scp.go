// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package copytool

import (
	"context"
	"errors"
	"fmt"
	"os/exec"

	"github.com/coreos/go-semver/semver"

	"github.com/lima-vm/lima/v2/pkg/limatype"
	"github.com/lima-vm/lima/v2/pkg/sshutil"
)

type scpTool struct {
	toolPath string
	Options  *Options
}

func newSCPTool(opts *Options) (*scpTool, error) {
	path, err := exec.LookPath("scp")
	if err != nil {
		return nil, fmt.Errorf("scp not found on host: %w", err)
	}
	return &scpTool{toolPath: path, Options: opts}, nil
}

func (t *scpTool) Name() string {
	return t.toolPath
}

func (t *scpTool) IsAvailableOnGuest(_ context.Context, _ []string) bool {
	// scp is typically available on all systems with SSH
	return true
}

func (t *scpTool) Command(ctx context.Context, paths []string, opts *Options) (*exec.Cmd, error) {
	copyPaths, err := parseCopyPaths(ctx, paths)
	if err != nil {
		return nil, err
	}

	if opts != nil {
		t.Options = opts
	}

	instances := make(map[string]*limatype.Instance)
	scpFlags := []string{}
	scpArgs := []string{}

	if t.Options.Verbose {
		scpFlags = append(scpFlags, "-v")
	} else {
		scpFlags = append(scpFlags, "-q")
	}

	if t.Options.Recursive {
		scpFlags = append(scpFlags, "-r")
	}

	if t.Options.AdditionalArgs != nil {
		scpFlags = append(scpFlags, t.Options.AdditionalArgs...)
	}

	// this assumes that ssh and scp come from the same place, but scp has no -V
	sshExeForVersion, err := sshutil.NewSSHExe()
	if err != nil {
		return nil, err
	}
	legacySSH := sshutil.DetectOpenSSHVersion(ctx, sshExeForVersion).LessThan(*semver.New("8.0.0"))

	for _, cp := range copyPaths {
		if cp.IsRemote {
			if legacySSH {
				scpFlags = append(scpFlags, "-P", fmt.Sprintf("%d", cp.Instance.SSHLocalPort))
				scpArgs = append(scpArgs, fmt.Sprintf("%s:%s", *cp.Instance.Config.User.Name+"@"+cp.Instance.SSHAddress, cp.Path))
			} else {
				scpArgs = append(scpArgs, fmt.Sprintf("scp://%s:%d/%s", *cp.Instance.Config.User.Name+"@"+cp.Instance.SSHAddress, cp.Instance.SSHLocalPort, cp.Path))
			}
			instances[cp.InstanceName] = cp.Instance
		} else {
			scpArgs = append(scpArgs, cp.Path)
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

	return exec.CommandContext(ctx, t.toolPath, append(sshArgs, scpArgs...)...), nil
}
