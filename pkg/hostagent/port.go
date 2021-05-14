package hostagent

import (
	"context"
	"strconv"

	"github.com/AkihiroSuda/lima/pkg/guestagent/api"
	"github.com/AkihiroSuda/sshocker/pkg/ssh"
	"github.com/sirupsen/logrus"
)

type portForwarder struct {
	sshConfig   *ssh.SSHConfig
	sshHostPort int
	tcp         map[int]struct{} // key: int (NOTE: this might be inconsistent with the actual status of SSH master)
}

const sshGuestPort = 22

func newPortForwarder(sshConfig *ssh.SSHConfig, sshHostPort int) *portForwarder {
	return &portForwarder{
		sshConfig:   sshConfig,
		sshHostPort: sshHostPort,
		tcp:         make(map[int]struct{}),
	}
}

func (pf *portForwarder) OnEvent(ctx context.Context, ev api.Event) {
	ignore := func(x api.IPPort) bool {
		switch x.Port {
		case sshGuestPort, pf.sshHostPort:
			return true
		default:
			return false
		}
	}
	for _, f := range ev.LocalPortsRemoved {
		if ignore(f) {
			continue
		}
		// pf.tcp might be inconsistent with the actual state of the SSH master,
		// so we always attempt to cancel forwarding, even when f.Port is not tracked in pf.tcp.
		logrus.Infof("Stopping forwarding TCP port %d", f.Port)
		verbCancel := true
		if err := forwardSSH(ctx, pf.sshConfig, pf.sshHostPort, "127.0.0.1:"+strconv.Itoa(f.Port), "127.0.0.1:"+strconv.Itoa(f.Port), verbCancel); err != nil {
			if _, ok := pf.tcp[f.Port]; ok {
				logrus.WithError(err).Warnf("failed to stop forwarding TCP port %d", f.Port)
			} else {
				logrus.WithError(err).Debugf("failed to stop forwarding TCP port %d (negligible)", f.Port)
			}
		}
		delete(pf.tcp, f.Port)
	}
	for _, f := range ev.LocalPortsAdded {
		if ignore(f) {
			continue
		}
		logrus.Infof("Forwarding TCP port %d", f.Port)
		if err := forwardSSH(ctx, pf.sshConfig, pf.sshHostPort, "127.0.0.1:"+strconv.Itoa(f.Port), "127.0.0.1:"+strconv.Itoa(f.Port), false); err != nil {
			logrus.WithError(err).Warnf("failed to setting up forward TCP port %d (negligible if already forwarded)", f.Port)
		} else {
			pf.tcp[f.Port] = struct{}{}
		}
	}
}
