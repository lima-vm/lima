// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package hostagent

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/lima-vm/sshocker/pkg/reversesshfs"
	"github.com/sirupsen/logrus"

	"github.com/lima-vm/lima/v2/pkg/limatype"
	"github.com/lima-vm/lima/v2/pkg/sshutil"
)

type mount struct {
	close func() error
}

// mountPathAndSftpServer returns location in the native path form, and the
// sftp-server from sshExe's toolchain. Every driver gets the native form
// because sshocker rewrites a Cygwin path to backslashes, which a Cygwin
// ssh.exe then eats, leaving the server a path it cannot resolve. Both server
// flavors accept the native form. The server is empty for the builtin driver,
// which sshocker serves in-process, and when the toolchain ships none.
func mountPathAndSftpServer(ctx context.Context, sshExe sshutil.SSHExe, driver, location string) (resolvedLocation, sftpServerBinary string) {
	resolvedLocation = filepath.ToSlash(location)
	if driver == limatype.SFTPDriverBuiltin {
		return resolvedLocation, ""
	}
	sftpServerBinary = sshutil.SftpServerForSSH(ctx, sshExe)
	if sftpServerBinary == "" {
		logrus.Infof("reverse-sshfs: no sftp-server in the toolchain of %#q; falling back to a generic search, which may pick one that cannot resolve %#q", sshExe.Exe, resolvedLocation)
	} else {
		logrus.Debugf("reverse-sshfs: host path %#q resolved to %#q for sftp-server %#q", location, resolvedLocation, sftpServerBinary)
	}
	return resolvedLocation, sftpServerBinary
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
	logrus.Infof("Mounting %#q on %#q", m.Location, *m.MountPoint)

	resolvedLocation := m.Location
	var sftpServerBinary string
	if runtime.GOOS == "windows" {
		resolvedLocation, sftpServerBinary = mountPathAndSftpServer(ctx, a.sshExe, *m.SSHFS.SFTPDriver, m.Location)
	}

	sshAddress, sshPort := a.sshAddressPort()
	// Create a copy of sshConfig to avoid
	// modifying HostAgent's sshConfig
	sshConfig := *a.sshConfig
	rsf := &reversesshfs.ReverseSSHFS{
		Driver:                  *m.SSHFS.SFTPDriver,
		OpensshSftpServerBinary: sftpServerBinary,
		SSHConfig:               &sshConfig,
		LocalPath:               resolvedLocation,
		Host:                    sshAddress,
		Port:                    sshPort,
		RemotePath:              *m.MountPoint,
		Readonly:                !(*m.Writable),
		SSHFSAdditionalArgs:     []string{"-o", sshfsOptions},
	}
	// Give the mount its own ssh connection so that it keeps working after the ControlMaster
	// exits (`limactl shell --reconnect` ends it).
	// Cygwin and MSYS2 ssh on Windows also lack part of the feature set over the mux socket.
	rsf.SSHConfig.AdditionalArgs = sshutil.DisableControlMasterOptsFromSSHArgs(rsf.SSHConfig.AdditionalArgs)
	if err := rsf.Prepare(); err != nil {
		return nil, fmt.Errorf("failed to prepare reverse sshfs for %#q on %#q: %w", resolvedLocation, *m.MountPoint, err)
	}
	if err := rsf.Start(); err != nil {
		logrus.WithError(err).Warnf("failed to mount reverse sshfs for %#q on %#q, retrying with `-o nonempty`", resolvedLocation, *m.MountPoint)
		// NOTE: nonempty is not supported for libfuse3: https://github.com/canonical/multipass/issues/1381
		rsf.SSHFSAdditionalArgs = []string{"-o", "nonempty"}
		if err := rsf.Start(); err != nil {
			return nil, fmt.Errorf("failed to mount reverse sshfs for %#q on %#q: %w", resolvedLocation, *m.MountPoint, err)
		}
	}

	res := &mount{
		close: func() error {
			logrus.Infof("Unmounting %#q", resolvedLocation)
			if err := rsf.Close(); err != nil {
				return fmt.Errorf("failed to unmount reverse sshfs for %#q on %#q: %w", resolvedLocation, *m.MountPoint, err)
			}
			return nil
		},
	}
	return res, nil
}
