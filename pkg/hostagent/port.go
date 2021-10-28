package hostagent

import (
	"context"
	"fmt"
	"net"

	"github.com/lima-vm/lima/pkg/guestagent/api"
	"github.com/lima-vm/lima/pkg/limayaml"
	"github.com/lima-vm/sshocker/pkg/ssh"
	"github.com/sirupsen/logrus"
)

type portForwarder struct {
	sshConfig   *ssh.SSHConfig
	sshHostPort int
	tcp         map[int]struct{} // key: int (NOTE: this might be inconsistent with the actual status of SSH master)
	unix        map[string]struct{}
	rules       []limayaml.PortForward
}

const sshGuestPort = 22

func newPortForwarder(sshConfig *ssh.SSHConfig, sshHostPort int, rules []limayaml.PortForward) *portForwarder {
	return &portForwarder{
		sshConfig:   sshConfig,
		sshHostPort: sshHostPort,
		tcp:         make(map[int]struct{}),
		unix:        make(map[string]struct{}),
		rules:       rules,
	}
}

func hostAddress(rule limayaml.PortForward, guest api.IPPort) string {
	if rule.HostSocket != "" {
		return rule.HostSocket
	}
	host := api.IPPort{
		IP:   rule.HostIP,
		Port: guest.Port + rule.HostPortRange[0] - rule.GuestPortRange[0],
	}
	return host.String()
}

func (pf *portForwarder) forwardingAddresses(guest api.IPPort, socket string) (string, string) {
	for _, rule := range pf.rules {
		if socket != "" {
			if socket == rule.GuestSocket {
				return hostAddress(rule, guest), socket
			}
			continue
		}
		if rule.GuestSocket != "" {
			continue
		}
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
		return hostAddress(rule, guest), guest.String()
	}
	return "", guest.String()
}

func portOrSocket(guest api.IPPort, socket string) string {
	if socket == "" {
		return fmt.Sprintf("tcp port %d", guest.Port)
	}
	return fmt.Sprintf("unix socket %q", socket)
}

func (pf *portForwarder) removeForward(ctx context.Context, guest api.IPPort, socket string) {
	// pf.tcp might be inconsistent with the actual state of the SSH master,
	// so we always attempt to cancel forwarding, even when f.Port is not tracked in pf.tcp.
	local, remote := pf.forwardingAddresses(guest, socket)
	if local == "" {
		return
	}
	logrus.Infof("Stopping forwarding from %s to %s", remote, local)
	verbCancel := true
	if err := forwardTCP(ctx, pf.sshConfig, pf.sshHostPort, local, remote, verbCancel); err != nil {
		_, okPort := pf.tcp[guest.Port]
		_, okSocket := pf.unix[socket]
		negligibile := " (negligibile)"
		if okPort || okSocket {
			negligibile = ""
		}
		logrus.WithError(err).Warnf("failed to stop forwarding %s%s", portOrSocket(guest, socket), negligibile)
	}
	if socket == "" {
		delete(pf.tcp, guest.Port)
	} else {
		delete(pf.unix, socket)
	}
}

func (pf *portForwarder) addForward(ctx context.Context, guest api.IPPort, socket string) {
	local, remote := pf.forwardingAddresses(guest, socket)
	if local == "" {
		logrus.Debugf("Not forwarding %s", portOrSocket(guest, socket))
		return
	}
	logrus.Infof("Forwarding from %s to %s", remote, local)
	if err := forwardTCP(ctx, pf.sshConfig, pf.sshHostPort, local, remote, false); err != nil {
		logrus.WithError(err).Warnf("failed to set up forwarding %s (negligible if already forwarded)",
			portOrSocket(guest, socket))
	} else {
		if socket == "" {
			pf.tcp[guest.Port] = struct{}{}
		} else {
			pf.unix[socket] = struct{}{}
		}
	}
}

func (pf *portForwarder) OnEvent(ctx context.Context, ev api.Event) {
	for _, f := range ev.LocalPortsRemoved {
		pf.removeForward(ctx, f, "")
	}
	for _, f := range ev.LocalPortsAdded {
		pf.addForward(ctx, f, "")
	}
	for _, f := range ev.LocalSocketsRemoved {
		pf.removeForward(ctx, api.IPPort{}, f)
	}
	for _, f := range ev.LocalSocketsAdded {
		pf.addForward(ctx, api.IPPort{}, f)
	}
}
