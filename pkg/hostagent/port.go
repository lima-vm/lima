package hostagent

import (
	"context"
	"net"

	"github.com/lima-vm/lima/pkg/guestagent/api"
	"github.com/lima-vm/lima/pkg/limayaml"
	"github.com/lima-vm/sshocker/pkg/ssh"
	"github.com/sirupsen/logrus"
)

type portForwarder struct {
	sshConfig   *ssh.SSHConfig
	sshHostPort int
	rules       []limayaml.PortForward
}

const sshGuestPort = 22

func newPortForwarder(sshConfig *ssh.SSHConfig, sshHostPort int, rules []limayaml.PortForward) *portForwarder {
	return &portForwarder{
		sshConfig:   sshConfig,
		sshHostPort: sshHostPort,
		rules:       rules,
	}
}

func hostAddress(rule limayaml.PortForward, guest api.IPPort) string {
	if rule.HostSocket != "" {
		return rule.HostSocket
	}
	host := api.IPPort{IP: rule.HostIP}
	if guest.Port == 0 {
		// guest is a socket
		host.Port = rule.HostPort
	} else {
		host.Port = guest.Port + rule.HostPortRange[0] - rule.GuestPortRange[0]
	}
	return host.String()
}

func (pf *portForwarder) forwardingAddresses(guest api.IPPort) (hostAddr string, guestAddr string) {
	// Some rules will require a small patch to the HostIP in order to bind to the
	// correct IP family.
	mustAdjustHostIP := false

	// This holds an optional rule that was rejected, but is now stored here to preserve backward
	// compatibility, and will be used at the bottom of this function if set. See the case
	// rule.GuestIPMustBeZero && guest.IP.IsUnspecified() for further info.
	var unspecifiedRuleFallback *limayaml.PortForward

	for _, rule := range pf.rules {
		if rule.GuestSocket != "" {
			// Not TCP
			continue
		}

		// Check if `guest.Port` is within `rule.GuestPortRange`
		if guest.Port < rule.GuestPortRange[0] || guest.Port > rule.GuestPortRange[1] {
			continue
		}

		switch {
		// Early-continue in case rule's IP is not zero while it is required.
		case rule.GuestIPMustBeZero && !guest.IP.IsUnspecified():
			continue

		// Rule lacks a preferred GuestIP, so guest may be binding to wherever it wants. The rule matches the port range,
		// so we can continue processing it. However, make sure to correct the rule to use a correct address family if
		// not specified by the rule.
		case rule.GuestIPWasUndefined && !rule.GuestIPMustBeZero:
			mustAdjustHostIP = rule.HostIPWasUndefined

		// if GuestIP and family matches, move along.
		case rule.GuestIPMustBeZero && guest.IP.IsUnspecified():
			// This is a breaking change. Here we will keep a backup of the rule, so we can still reuse it
			// in case everything fails. The idea here is to move a copy of the current rule to outside this
			// loop, so we can reuse it in case nothing else matches.
			if !rule.GuestIPWasUndefined && !guest.IP.Equal(rule.GuestIP) {
				if unspecifiedRuleFallback == nil {
					// Move the rule to obtain a copy
					func(p limayaml.PortForward) { unspecifiedRuleFallback = &p }(rule)
				}
				continue
			}

			mustAdjustHostIP = rule.HostIPWasUndefined

		// Rule lack's HostIP, and guest is binding to '0.0.0.0' or '::'. Bind to the same address family.
		case rule.HostIPWasUndefined && guest.IP.IsUnspecified():
			mustAdjustHostIP = true

		// We don't have a preferred HostIP in the rule, and guest wants to bind to a loopback
		// address. In that case, use the same address family.
		case rule.HostIPWasUndefined && (guest.IP.Equal(net.IPv6loopback) || guest.IP.Equal(api.IPv4loopback1)):
			mustAdjustHostIP = true

		case guest.IP.Equal(rule.GuestIP):
		case guest.IP.Equal(net.IPv6loopback) && rule.GuestIP.Equal(api.IPv4loopback1):
		case rule.GuestIP.IsUnspecified() && !rule.GuestIPMustBeZero:
			// When GuestIPMustBeZero is true, then 0.0.0.0 must be an exact match, which is already
			// handled above by the guest.IP.IsUnspecified() condition.
		default:
			continue
		}

		if rule.Ignore {
			if guest.IP.IsUnspecified() && !rule.GuestIP.IsUnspecified() {
				continue
			}

			break
		}

		if mustAdjustHostIP {
			if guest.IP.To4() != nil {
				rule.HostIP = api.IPv4loopback1
			} else {
				rule.HostIP = net.IPv6loopback
			}
		}

		return hostAddress(rule, guest), guest.String()
	}

	// At this point, no other rule matched. So check if this is being impacted by our
	// breaking change, and return the fallback rule. Otherwise, just ignore it.
	if unspecifiedRuleFallback != nil {
		return hostAddress(*unspecifiedRuleFallback, guest), guest.String()
	}

	return "", guest.String()
}

func (pf *portForwarder) OnEvent(ctx context.Context, ev api.Event) {
	for _, f := range ev.LocalPortsRemoved {
		local, remote := pf.forwardingAddresses(f)
		if local == "" {
			continue
		}
		logrus.Infof("Stopping forwarding TCP from %s to %s", remote, local)
		if err := forwardTCP(ctx, pf.sshConfig, pf.sshHostPort, local, remote, verbCancel); err != nil {
			logrus.WithError(err).Warnf("failed to stop forwarding tcp port %d", f.Port)
		}
	}
	for _, f := range ev.LocalPortsAdded {
		local, remote := pf.forwardingAddresses(f)
		if local == "" {
			logrus.Infof("Not forwarding TCP %s", remote)
			continue
		}
		logrus.Infof("Forwarding TCP from %s to %s", remote, local)
		if err := forwardTCP(ctx, pf.sshConfig, pf.sshHostPort, local, remote, verbForward); err != nil {
			logrus.WithError(err).Warnf("failed to set up forwarding tcp port %d (negligible if already forwarded)", f.Port)
		}
	}
}
