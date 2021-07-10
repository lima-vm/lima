package hostagent

import (
	"context"
	"fmt"
	"strconv"

	"github.com/AkihiroSuda/lima/pkg/guestagent/api"
	"github.com/AkihiroSuda/lima/pkg/limayaml"
	"github.com/AkihiroSuda/sshocker/pkg/ssh"
	"github.com/sirupsen/logrus"
)

type portForwarder struct {
	l           *logrus.Logger
	sshConfig   *ssh.SSHConfig
	sshHostPort int
	tcp         map[int]struct{} // key: int (NOTE: this might be inconsistent with the actual status of SSH master)
	ports       []limayaml.Port
}

const sshGuestPort = 22

func newPortForwarder(l *logrus.Logger, sshConfig *ssh.SSHConfig, sshHostPort int, ports []limayaml.Port) *portForwarder {
	return &portForwarder{
		l:           l,
		sshConfig:   sshConfig,
		sshHostPort: sshHostPort,
		tcp:         make(map[int]struct{}),
		ports:       ports,
	}
}

func (pf *portForwarder) forwardingAddresses(guest api.IPPort) (string, string) {
	for _, port := range pf.ports {
		if port.GuestPortRange[0] <= guest.Port && guest.Port <= port.GuestPortRange[1] {
			guestAddr := fmt.Sprintf("%s:%d", port.GuestIP, guest.Port)
			if port.Ignore {
				return guestAddr, ""
			}
			offset := port.HostPortRange[0] - port.GuestPortRange[0]
			hostAddr := fmt.Sprintf("%s:%d", port.HostIP, guest.Port + offset)
			return guestAddr, hostAddr
		}
	}
	addr := "127.0.0.1:" + strconv.Itoa(guest.Port)
	return addr, addr
}

func (pf *portForwarder) OnEvent(ctx context.Context, ev api.Event) {
	for _, f := range ev.LocalPortsRemoved {
		// pf.tcp might be inconsistent with the actual state of the SSH master,
		// so we always attempt to cancel forwarding, even when f.Port is not tracked in pf.tcp.
		guestAddr, hostAddr := pf.forwardingAddresses(f)
		if hostAddr == "" {
			continue
		}
		pf.l.Infof("Stopping forwarding TCP from %s to %s", guestAddr, hostAddr)
		verbCancel := true
		if err := forwardSSH(ctx, pf.sshConfig, pf.sshHostPort, hostAddr, guestAddr, verbCancel); err != nil {
			if _, ok := pf.tcp[f.Port]; ok {
				pf.l.WithError(err).Warnf("failed to stop forwarding TCP port %d", f.Port)
			} else {
				pf.l.WithError(err).Debugf("failed to stop forwarding TCP port %d (negligible)", f.Port)
			}
		}
		delete(pf.tcp, f.Port)
	}
	for _, f := range ev.LocalPortsAdded {
		guestAddr, hostAddr := pf.forwardingAddresses(f)
		if hostAddr == "" {
			pf.l.Infof("Not forwarding TCP from %s", guestAddr)
			continue
		}
		pf.l.Infof("Forwarding TCP from %s to %s", guestAddr, hostAddr)
		if err := forwardSSH(ctx, pf.sshConfig, pf.sshHostPort, hostAddr, guestAddr, false); err != nil {
			pf.l.WithError(err).Warnf("failed to setting up forward TCP port %d (negligible if already forwarded)", f.Port)
		} else {
			pf.tcp[f.Port] = struct{}{}
		}
	}
}
