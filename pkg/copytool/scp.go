// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package copytool

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"

	"github.com/coreos/go-semver/semver"

	"github.com/lima-vm/lima/v2/pkg/limatype"
	"github.com/lima-vm/lima/v2/pkg/sshutil"
)

type scpTool struct {
	toolPath string
	sshExe   sshutil.SSHExe
	Options  *Options
}

func newSCPTool(opts *Options) (*scpTool, error) {
	// scp must come from the same toolchain as the ssh whose path form it
	// receives, and NewSSHExe can select an ssh that PATH would not.
	sshExe, err := sshutil.NewSSHExe()
	if err != nil {
		return nil, fmt.Errorf("ssh not found on host: %w", err)
	}
	path, err := exec.LookPath(sshutil.CompanionForSSH(sshExe, "scp"))
	if err != nil {
		return nil, fmt.Errorf("scp not found on host: %w", err)
	}
	return &scpTool{toolPath: path, sshExe: sshExe, Options: opts}, nil
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

	effectiveOpts := t.Options
	if opts != nil {
		effectiveOpts = opts
	}

	instances := make(map[string]*limatype.Instance)
	scpFlags := []string{}
	scpArgs := []string{}

	if effectiveOpts.Verbose {
		scpFlags = append(scpFlags, "-v")
	} else {
		scpFlags = append(scpFlags, "-q")
	}

	if effectiveOpts.Recursive {
		scpFlags = append(scpFlags, "-r")
	}

	if effectiveOpts.AdditionalArgs != nil {
		scpFlags = append(scpFlags, effectiveOpts.AdditionalArgs...)
	}

	// scp has no -V, so the version comes from t.sshExe, which is scp's own
	// toolchain unless that install shipped no scp.
	legacySSH := sshutil.DetectOpenSSHVersion(ctx, t.sshExe).LessThan(*semver.New("8.0.0"))

	// Only absolute paths carry a drive letter to convert; a relative path is
	// already in a form both toolchains accept. Key the form to scp itself,
	// which parses these operands: it usually comes from the same toolchain as
	// t.sshExe, but falls back to PATH when that install ships no scp.
	for _, cp := range copyPaths {
		if cp.IsRemote || !filepath.IsAbs(cp.Path) {
			continue
		}
		if cp.Path, err = sshutil.PathForTool(ctx, t.toolPath, cp.Path); err != nil {
			return nil, err
		}
	}

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
		// sessions without re-authenticating (MaxSessions permitting), except on
		// Windows.
		for _, inst := range instances {
			sshOpts, err = sshOptsForInstance(ctx, t.sshExe, t.toolPath, inst)
			if err != nil {
				return nil, err
			}
		}
	} else {
		// Copying among multiple hosts; we can't pass in host-specific options.
		// CommonOpts carries no multiplexing options to begin with.
		sshOpts, err = sshutil.CommonOpts(ctx, t.sshExe, t.toolPath, false)
		if err != nil {
			return nil, err
		}
	}
	sshArgs := sshutil.SSHArgsFromOpts(sshOpts)

	return exec.CommandContext(ctx, t.toolPath, append(sshArgs, scpArgs...)...), nil
}
