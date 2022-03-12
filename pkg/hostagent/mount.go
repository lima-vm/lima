package hostagent

import (
	"context"
	"fmt"
	"os"

	"github.com/docker/go-units"
	"github.com/hashicorp/go-multierror"

	"github.com/lima-vm/lima/pkg/limayaml"
	"github.com/lima-vm/lima/pkg/localpathutil"
	"github.com/lima-vm/lima/pkg/remotemount"
	"github.com/lima-vm/sshocker/pkg/reversesshfs"
	"github.com/sirupsen/logrus"
)

type mount struct {
	close func() error
}

func (a *HostAgent) setupMounts(ctx context.Context) ([]*mount, error) {
	var (
		res  []*mount
		mErr error
	)
	for i, f := range a.y.Mounts {
		m, err := a.setupMount(ctx, i, f)
		if err != nil {
			mErr = multierror.Append(mErr, err)
			continue
		}
		res = append(res, m)
	}
	return res, mErr
}

func (a *HostAgent) setupMount(ctx context.Context, i int, m limayaml.Mount) (*mount, error) {
	switch *a.y.MountType {
	case limayaml.REVSSHFS:
		return a.setupMountReverseSSHFS(ctx, m)
	case limayaml.NINEP:
		return a.setupMount9P(ctx, i, m)
	}
	return nil, fmt.Errorf("unknown mount type: %v", *a.y.MountType)
}

func (a *HostAgent) setupMountReverseSSHFS(ctx context.Context, m limayaml.Mount) (*mount, error) {
	expanded, err := localpathutil.Expand(m.Location)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(expanded, 0755); err != nil {
		return nil, err
	}
	// NOTE: allow_other requires "user_allow_other" in /etc/fuse.conf
	sshfsOptions := "allow_other"
	if !*m.SSHFS.Cache {
		sshfsOptions = sshfsOptions + ",cache=no"
	}
	if *m.SSHFS.FollowSymlinks {
		sshfsOptions = sshfsOptions + ",follow_symlinks"
	}
	logrus.Infof("Mounting %q", expanded)
	rsf := &reversesshfs.ReverseSSHFS{
		SSHConfig:           a.sshConfig,
		LocalPath:           expanded,
		Host:                "127.0.0.1",
		Port:                a.sshLocalPort,
		RemotePath:          expanded,
		Readonly:            !(*m.Writable),
		SSHFSAdditionalArgs: []string{"-o", sshfsOptions},
	}
	if err := rsf.Prepare(); err != nil {
		return nil, fmt.Errorf("failed to prepare reverse sshfs for %q: %w", expanded, err)
	}
	if err := rsf.Start(); err != nil {
		logrus.WithError(err).Warnf("failed to mount reverse sshfs for %q, retrying with `-o nonempty`", expanded)
		// NOTE: nonempty is not supported for libfuse3: https://github.com/canonical/multipass/issues/1381
		rsf.SSHFSAdditionalArgs = []string{"-o", "nonempty"}
		if err := rsf.Start(); err != nil {
			return nil, fmt.Errorf("failed to mount reverse sshfs for %q: %w", expanded, err)
		}
	}

	res := &mount{
		close: func() error {
			logrus.Infof("Unmounting %q", expanded)
			if closeErr := rsf.Close(); closeErr != nil {
				return fmt.Errorf("failed to unmount reverse sshfs for %q: %w", expanded, err)
			}
			return nil
		},
	}
	return res, nil
}

func (a *HostAgent) setupMount9P(ctx context.Context, i int, m limayaml.Mount) (*mount, error) {
	expanded, err := localpathutil.Expand(m.Location)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(expanded, 0755); err != nil {
		return nil, err
	}
	mountOptions := "trans=virtio"
	mountOptions += fmt.Sprintf(",version=%s", *m.NineP.ProtocolVersion)
	msize, err := units.RAMInBytes(*m.NineP.Msize)
	if err != nil {
		return nil, fmt.Errorf("failed to parse msize for %q: %w", expanded, err)
	}
	mountOptions += fmt.Sprintf(",msize=%d", msize)
	mountOptions += fmt.Sprintf(",cache=%s", *m.NineP.Cache)

	logrus.Infof("Mounting %q", expanded)
	rsf := &remotemount.RemoteMount{
		SSHConfig:           a.sshConfig,
		LocalPath:           expanded,
		Host:                "127.0.0.1",
		Port:                a.sshLocalPort,
		RemotePath:          expanded,
		Readonly:            !(*m.Writable),
		MountType:           "9p",
		MountTag:            fmt.Sprintf("mount%d", i),
		MountAdditionalArgs: []string{"-o", mountOptions},
	}
	if err := rsf.Prepare(); err != nil {
		return nil, fmt.Errorf("failed to prepare 9p for %q: %w", expanded, err)
	}
	if err := rsf.Start(); err != nil {
		return nil, fmt.Errorf("failed to mount 9p for %q: %w", expanded, err)
	}

	res := &mount{
		close: func() error {
			logrus.Infof("Unmounting %q", expanded)
			if closeErr := rsf.Close(); closeErr != nil {
				return fmt.Errorf("failed to unmount 9p for %q: %w", expanded, err)
			}
			return nil
		},
	}
	return res, nil
}
