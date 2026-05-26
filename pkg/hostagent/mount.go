// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package hostagent

import (
	"context"
	"errors"
	"fmt"
	"os"
	"runtime"

	"github.com/lima-vm/sshocker/pkg/reversesshfs"
	"github.com/sirupsen/logrus"

	"github.com/lima-vm/lima/v2/pkg/ioutilx"
	"github.com/lima-vm/lima/v2/pkg/limatype"
	"github.com/lima-vm/lima/v2/pkg/sshutil"
)

type mount struct {
	close func() error
}

func (a *HostAgent) setupMounts(ctx context.Context) ([]*mount, error) {
	var (
		res  []*mount
		errs []error
	)
	for _, f := range a.instConfig.Mounts {
		m, err := a.setupMount(ctx, f)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		res = append(res, m)
	}
	return res, errors.Join(errs...)
}

func (a *HostAgent) setupMount(ctx context.Context, m limatype.Mount) (*mount, error) {
	if err := os.MkdirAll(m.Location, 0o755); err != nil {
		return nil, err
	}
	// NOTE: allow_other requires "user_allow_other" in /etc/fuse.conf
	sshfsOptions := "allow_other"
	if !*m.SSHFS.Cache {
		sshfsOptions += ",cache=no"
	}
	if *m.SSHFS.FollowSymlinks {
		sshfsOptions += ",follow_symlinks"
	}
	logrus.Infof("Mounting %q on %q", m.Location, *m.MountPoint)

	resolvedLocation := m.Location
	if runtime.GOOS == "windows" {
		var err error
		resolvedLocation, err = ioutilx.WindowsSubsystemPath(ctx, m.Location)
		if err != nil {
			return nil, err
		}
	}

	sshAddress, sshPort := a.sshAddressPort()
	// Create a copy of sshConfig to avoid
	// modifying HostAgent's sshConfig in case of Windows
	sshConfig := *a.sshConfig
	rsf := &reversesshfs.ReverseSSHFS{
		Driver:              *m.SSHFS.SFTPDriver,
		SSHConfig:           &sshConfig,
		LocalPath:           resolvedLocation,
		Host:                sshAddress,
		Port:                sshPort,
		RemotePath:          *m.MountPoint,
		Readonly:            !(*m.Writable),
		SSHFSAdditionalArgs: []string{"-o", sshfsOptions},
	}
	if runtime.GOOS == "windows" {
		// cygwin/msys2 doesn't support full feature set over mux socket, this has at least 2 side effects:
		// 1. unnecessary pollutes output with error on errors encountered (ssh will try to tolerate them with fallbacks);
		// 2. these errors still imply additional coms over mux socket, which resulted sftp-server to fail more often statistically during test runs.
		// It is reasonable to disable this on Windows if required feature is not fully operational.
		rsf.SSHConfig.Persist = false

		// HostAgent's `sshConfig` already has some ControlMaster related args in `AdditionalArgs`,
		// so it is necessary to remove them to avoid overrides above `Persist=false`.
		rsf.SSHConfig.AdditionalArgs = sshutil.DisableControlMasterOptsFromSSHArgs(rsf.SSHConfig.AdditionalArgs)
	}
	if err := rsf.Prepare(); err != nil {
		return nil, fmt.Errorf("failed to prepare reverse sshfs for %q on %q: %w", resolvedLocation, *m.MountPoint, err)
	}
	if err := rsf.Start(); err != nil {
		logrus.WithError(err).Warnf("failed to mount reverse sshfs for %q on %q, retrying with `-o nonempty`", resolvedLocation, *m.MountPoint)
		// NOTE: nonempty is not supported for libfuse3: https://github.com/canonical/multipass/issues/1381
		rsf.SSHFSAdditionalArgs = []string{"-o", "nonempty"}
		if err := rsf.Start(); err != nil {
			return nil, fmt.Errorf("failed to mount reverse sshfs for %q on %q: %w", resolvedLocation, *m.MountPoint, err)
		}
	}

	res := &mount{
		close: func() error {
			logrus.Infof("Unmounting %q", resolvedLocation)
			if err := rsf.Close(); err != nil {
				return fmt.Errorf("failed to unmount reverse sshfs for %q on %q: %w", resolvedLocation, *m.MountPoint, err)
			}
			return nil
		},
	}
	return res, nil
}
