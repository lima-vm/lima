package hostagent

import (
	"context"
	"net"

	"github.com/lima-vm/sshocker/pkg/ssh"
	"github.com/lima-vm/lima/pkg/guestagent/api"
	"github.com/lima-vm/lima/pkg/limayaml"
	"github.com/sirupsen/logrus"
)

type portForwarder struct {
	l           *logrus.Logger
	sshConfig   *ssh.SSHConfig
	sshHostPort int
	tcp         map[int]struct{} // key: int (NOTE: this might be inconsistent with the actual status of SSH master)
	rules       []limayaml.PortForward
}

const sshGuestPort = 22

func newPortForwarder(l *logrus.Logger, sshConfig *ssh.SSHConfig, sshHostPort int, rules []limayaml.PortForward) *portForwarder {
	return &portForwarder{
		l:           l,
		sshConfig:   sshConfig,
		sshHostPort: sshHostPort,
		tcp:         make(map[int]struct{}),
		rules:       rules,
	}
}

func (pf *portForwarder) forwardingAddresses(guest api.IPPort) (string, string) {
	for _, rule := range pf.rules {
		if guest.Port < rule.GuestPortRange[0] || guest.Port > rule.GuestPortRange[1] {
			continue
		}
		switch {
		case guest.IP.IsUnspecified():
		case guest.IP.Equal(rule.GuestIP):
		case guest.IP.Equal(net.IPv6loopback) && rule.GuestIP.Equal(api.IPv4loopback1):
		case rule.GuestIP.IsUnspecified():
		default:
			continue
		}
		if rule.Ignore {
			if guest.IP.IsUnspecified() && !rule.GuestIP.IsUnspecified() {
				continue
			}
			break
		}
		host := api.IPPort{
			IP:   rule.HostIP,
			Port: guest.Port + rule.HostPortRange[0] - rule.GuestPortRange[0],
		}
		return host.String(), guest.String()
	}
	return "", guest.String()
}

func (pf *portForwarder) OnEvent(ctx context.Context, ev api.Event) {
	for _, f := range ev.LocalPortsRemoved {
		// pf.tcp might be inconsistent with the actual state of the SSH master,
		// so we always attempt to cancel forwarding, even when f.Port is not tracked in pf.tcp.
		local, remote := pf.forwardingAddresses(f)
		if local == "" {
			continue
		}
		pf.l.Infof("Stopping forwarding TCP from %s to %s", remote, local)
		verbCancel := true
		if err := forwardSSH(ctx, pf.sshConfig, pf.sshHostPort, local, remote, verbCancel); err != nil {
			if _, ok := pf.tcp[f.Port]; ok {
				pf.l.WithError(err).Warnf("failed to stop forwarding TCP port %d", f.Port)
			} else {
				pf.l.WithError(err).Debugf("failed to stop forwarding TCP port %d (negligible)", f.Port)
			}
		}
		delete(pf.tcp, f.Port)
	}
	for _, f := range ev.LocalPortsAdded {
		local, remote := pf.forwardingAddresses(f)
		if local == "" {
			pf.l.Infof("Not forwarding TCP %s", remote)
			continue
		}
		pf.l.Infof("Forwarding TCP from %s to %s", remote, local)
		if err := forwardSSH(ctx, pf.sshConfig, pf.sshHostPort, local, remote, false); err != nil {
			pf.l.WithError(err).Warnf("failed to set up forwarding TCP port %d (negligible if already forwarded)", f.Port)
		} else {
			pf.tcp[f.Port] = struct{}{}
		}
	}
}
