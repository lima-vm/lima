// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package hostagent

import (
	"context"
	"net"

	"github.com/lima-vm/sshocker/pkg/ssh"
	"github.com/sirupsen/logrus"

	"github.com/lima-vm/lima/v2/pkg/guestagent/api"
	"github.com/lima-vm/lima/v2/pkg/hostagent/events"
	"github.com/lima-vm/lima/v2/pkg/limatype"
	"github.com/lima-vm/lima/v2/pkg/limayaml"
	"github.com/lima-vm/lima/v2/pkg/ptr"
)

type portForwarder struct {
	sshConfig      *ssh.SSHConfig
	sshAddressPort func() (string, int)
	rules          []limatype.PortForward
	ignore         bool
	vmType         limatype.VMType
	onEvent        func(*events.PortForwardEvent)
}

const sshGuestPort = 22

var IPv4loopback1 = limayaml.IPv4loopback1

// portForwardRules returns the rules used by both forwarders: the SSH ports are
// blocked on every guest IP, then the instance rules, then the default forward
// for the remaining ports.
func portForwardRules(instDir string, user limatype.User, param map[string]string, sshLocalPort int, instRules []limatype.PortForward) []limatype.PortForward {
	rules := make([]limatype.PortForward, 0, 3+len(instRules))
	for _, port := range []int{sshGuestPort, sshLocalPort} {
		// GuestIPMustBeZero must be set explicitly. FillPortForwardDefaults would otherwise
		// derive it from GuestIP and turn the rule into an exact 0.0.0.0 match, which leaves
		// a guest listening on 127.0.0.1 or ::1 unblocked.
		rule := limatype.PortForward{GuestIP: net.IPv4zero, GuestIPMustBeZero: ptr.Of(false), GuestPort: port, Ignore: true}
		limayaml.FillPortForwardDefaults(&rule, instDir, user, param)
		rules = append(rules, rule)
	}
	rules = append(rules, instRules...)
	// Default forwards for all non-privileged ports from "127.0.0.1" and "::1"
	rule := limatype.PortForward{}
	limayaml.FillPortForwardDefaults(&rule, instDir, user, param)
	return append(rules, rule)
}

func newPortForwarder(sshConfig *ssh.SSHConfig, sshAddressPort func() (string, int), rules []limatype.PortForward, ignore bool, vmType limatype.VMType, onEvent func(*events.PortForwardEvent)) *portForwarder {
	return &portForwarder{
		sshConfig:      sshConfig,
		sshAddressPort: sshAddressPort,
		rules:          rules,
		ignore:         ignore,
		vmType:         vmType,
		onEvent:        onEvent,
	}
}

func (pf *portForwarder) emitEvent(ev *events.PortForwardEvent) {
	if pf.onEvent != nil {
		pf.onEvent(ev)
	}
}

func hostAddress(rule limatype.PortForward, guest *api.IPPort) string {
	if rule.HostSocket != "" {
		return rule.HostSocket
	}
	host := &api.IPPort{Ip: rule.HostIP.String()}
	if guest.Port == 0 {
		// guest is a socket
		host.Port = int32(rule.HostPort)
	} else {
		host.Port = guest.Port + int32(rule.HostPortRange[0]-rule.GuestPortRange[0])
	}
	return host.HostString()
}

func (pf *portForwarder) forwardingAddresses(guest *api.IPPort) (hostAddr, guestAddr string) {
	guestIP := net.ParseIP(guest.Ip)
	for _, rule := range pf.rules {
		if rule.GuestSocket != "" {
			continue
		}
		switch rule.Proto {
		case limatype.ProtoTCP, limatype.ProtoAny:
		default:
			continue
		}
		if guest.Port < int32(rule.GuestPortRange[0]) || guest.Port > int32(rule.GuestPortRange[1]) {
			continue
		}
		switch {
		case guestIP.IsUnspecified():
		case guestIP.Equal(rule.GuestIP):
		case guestIP.Equal(net.IPv6loopback) && rule.GuestIP.Equal(IPv4loopback1):
		case rule.GuestIP.IsUnspecified() && !*rule.GuestIPMustBeZero:
			// When GuestIPMustBeZero is true, then 0.0.0.0 must be an exact match, which is already
			// handled above by the guest.IP.IsUnspecified() condition.
		default:
			continue
		}
		if rule.Ignore {
			if guestIP.IsUnspecified() && !rule.GuestIP.IsUnspecified() {
				continue
			}
			break
		}
		return hostAddress(rule, guest), guest.HostString()
	}
	return "", guest.HostString()
}

func (pf *portForwarder) OnEvent(ctx context.Context, ev *api.Event) {
	sshAddress, sshPort := pf.sshAddressPort()
	for _, f := range ev.RemovedLocalPorts {
		if f.Protocol != "tcp" {
			continue
		}
		local, remote := pf.forwardingAddresses(f)
		if local == "" {
			continue
		}
		logrus.Infof("Stopping forwarding TCP from %s to %s", remote, local)
		pf.emitEvent(&events.PortForwardEvent{
			Type:      events.PortForwardEventStopping,
			Protocol:  "tcp",
			GuestAddr: remote,
			HostAddr:  local,
		})
		if err := forwardTCP(ctx, pf.sshConfig, sshAddress, sshPort, local, remote, verbCancel); err != nil {
			logrus.WithError(err).Warnf("failed to stop forwarding tcp port %d", f.Port)
		}
	}
	for _, f := range ev.AddedLocalPorts {
		if f.Protocol != "tcp" {
			continue
		}
		local, remote := pf.forwardingAddresses(f)
		if local == "" {
			if !pf.ignore {
				logrus.Infof("Not forwarding TCP %s", remote)
				pf.emitEvent(&events.PortForwardEvent{
					Type:      events.PortForwardEventNotForwarding,
					Protocol:  "tcp",
					GuestAddr: remote,
				})
			}
			continue
		}
		logrus.Infof("Forwarding TCP from %s to %s", remote, local)
		pf.emitEvent(&events.PortForwardEvent{
			Type:      events.PortForwardEventForwarding,
			Protocol:  "tcp",
			GuestAddr: remote,
			HostAddr:  local,
		})
		if err := forwardTCP(ctx, pf.sshConfig, sshAddress, sshPort, local, remote, verbForward); err != nil {
			logrus.WithError(err).Warnf("failed to set up forwarding tcp port %d (negligible if already forwarded)", f.Port)
		}
	}
}
