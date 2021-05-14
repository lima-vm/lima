package hostagent

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"

	"github.com/AkihiroSuda/lima/pkg/guestagent/api"
	"github.com/AkihiroSuda/lima/pkg/guestagent/api/client"
	"github.com/AkihiroSuda/lima/pkg/limayaml"
	"github.com/AkihiroSuda/lima/pkg/sshutil"
	"github.com/AkihiroSuda/sshocker/pkg/ssh"
	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

type HostAgent struct {
	y             *limayaml.LimaYAML
	instDir       string
	sshConfig     *ssh.SSHConfig
	portForwarder *portForwarder
	onClose       []func() error // LIFO
}

func New(y *limayaml.LimaYAML, instDir string) (*HostAgent, error) {
	sshArgs, err := sshutil.SSHArgs(instDir)
	if err != nil {
		return nil, err
	}
	sshConfig := &ssh.SSHConfig{
		AdditionalArgs: sshArgs,
	}
	a := &HostAgent{
		y:             y,
		instDir:       instDir,
		sshConfig:     sshConfig,
		portForwarder: newPortForwarder(sshConfig, y.SSH.LocalPort),
	}
	return a, nil
}

func (a *HostAgent) Run(ctx context.Context) error {
	a.onClose = append(a.onClose, func() error {
		logrus.Debugf("shutting down the SSH master")
		if exitMasterErr := ssh.ExitMaster("127.0.0.1", a.y.SSH.LocalPort, a.sshConfig); exitMasterErr != nil {
			logrus.WithError(exitMasterErr).Warn("failed to exit SSH master")
		}
		return nil
	})
	var mErr error
	if err := a.waitForRequirements(ctx, "essential", essentialRequirements); err != nil {
		mErr = multierror.Append(mErr, err)
	}
	mounts, err := a.setupMounts(ctx)
	if err != nil {
		mErr = multierror.Append(mErr, err)
	}
	a.onClose = append(a.onClose, func() error {
		var unmountMErr error
		for _, m := range mounts {
			if unmountErr := m.close(); unmountErr != nil {
				unmountMErr = multierror.Append(unmountMErr, unmountErr)
			}
		}
		return unmountMErr
	})
	go a.watchGuestAgentEvents(ctx)
	if err := a.waitForRequirements(ctx, "optional", optionalRequirements); err != nil {
		mErr = multierror.Append(mErr, err)
	}
	return mErr
}

func (a *HostAgent) Close() error {
	logrus.Infof("Shutting down the host agent")
	var mErr error
	for i := len(a.onClose) - 1; i >= 0; i-- {
		f := a.onClose[i]
		if err := f(); err != nil {
			mErr = multierror.Append(mErr, err)
		}
	}
	return mErr
}

func (a *HostAgent) watchGuestAgentEvents(ctx context.Context) {
	// TODO: use vSock (when QEMU for macOS gets support for vSock)

	localUnix := filepath.Join(a.instDir, "ga.sock")
	// guest should have same UID as the host (specified in cidata)
	remoteUnix := fmt.Sprintf("/run/user/%d/lima-guestagent.sock", os.Getuid())

	for {
		if !isGuestAgentSocketAccessible(ctx, localUnix) {
			if err := os.RemoveAll(localUnix); err != nil {
				logrus.WithError(err).Warnf("failed to clean up %q (host) before setting up forwarding", localUnix)
			}
			logrus.Infof("Forwarding %q (guest) to %q (host)", remoteUnix, localUnix)
			if err := forwardSSH(ctx, a.sshConfig, a.y.SSH.LocalPort, localUnix, remoteUnix, false); err != nil {
				logrus.WithError(err).Warnf("failed to setting up forward from %q (guest) to %q (host)", remoteUnix, localUnix)
			}
		}
		if err := a.processGuestAgentEvents(ctx, localUnix); err != nil {
			logrus.WithError(err).Warn("connection to the guest agent was closed unexpectedly")
		}
		select {
		case <-ctx.Done():
			logrus.Infof("Stopping forwarding %q to %q", remoteUnix, localUnix)
			verbCancel := true
			if err := forwardSSH(ctx, a.sshConfig, a.y.SSH.LocalPort, localUnix, remoteUnix, verbCancel); err != nil {
				logrus.WithError(err).Warnf("failed to stop forwarding %q (remote) to %q (local)", remoteUnix, localUnix)
			}
			if err := os.RemoveAll(localUnix); err != nil {
				logrus.WithError(err).Warnf("failed to clean up %q (host) after stopping forwarding", localUnix)
			}
			return
		case <-time.After(10 * time.Second):
		}
	}
}

func isGuestAgentSocketAccessible(ctx context.Context, localUnix string) bool {
	client, err := client.NewGuestAgentClient(localUnix)
	if err != nil {
		return false
	}
	_, err = client.Info(ctx)
	return err == nil
}

func (a *HostAgent) processGuestAgentEvents(ctx context.Context, localUnix string) error {
	client, err := client.NewGuestAgentClient(localUnix)
	if err != nil {
		return err
	}

	info, err := client.Info(ctx)
	if err != nil {
		return err
	}

	logrus.Debugf("guest agent info: %+v", info)

	onEvent := func(ev api.Event) {
		logrus.Debugf("guest agent event: %+v", ev)
		for _, f := range ev.Errors {
			logrus.Warnf("received error from the guest: %q", f)
		}
		a.portForwarder.OnEvent(ctx, ev)
	}

	if err := client.Events(ctx, onEvent); err != nil {
		return err
	}
	return io.EOF
}

func forwardSSH(ctx context.Context, sshConfig *ssh.SSHConfig, port int, local, remote string, cancel bool) error {
	args := sshConfig.Args()
	verb := "forward"
	if cancel {
		verb = "cancel"
	}
	args = append(args,
		"-T",
		"-O", verb,
		"-L", local+":"+remote,
		"-N",
		"-f",
		"-p", strconv.Itoa(port),
		"127.0.0.1",
		"--",
	)
	cmd := exec.CommandContext(ctx, sshConfig.Binary(), args...)
	if out, err := cmd.Output(); err != nil {
		return errors.Wrapf(err, "failed to run %v: %q", cmd.Args, string(out))
	}
	return nil
}
