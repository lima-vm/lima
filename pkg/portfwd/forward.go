package portfwd

import (
	"context"
	"net"
	"strings"

	"github.com/lima-vm/lima/pkg/guestagent/api"
	guestagentclient "github.com/lima-vm/lima/pkg/guestagent/api/client"
	"github.com/lima-vm/lima/pkg/limayaml"
	"github.com/sirupsen/logrus"
)

var IPv4loopback1 = limayaml.IPv4loopback1

type Forwarder struct {
	rules             []limayaml.PortForward
	ignoreTCP         bool
	ignoreUDP         bool
	closableListeners *ClosableListeners
}

func NewPortForwarder(rules []limayaml.PortForward, ignoreTCP, ignoreUDP bool) *Forwarder {
	return &Forwarder{
		rules:             rules,
		ignoreTCP:         ignoreTCP,
		ignoreUDP:         ignoreUDP,
		closableListeners: NewClosableListener(),
	}
}

func (fw *Forwarder) OnEvent(ctx context.Context, client *guestagentclient.GuestAgentClient, ev *api.Event) {
	for _, f := range ev.LocalPortsAdded {
		local, remote := fw.forwardingAddresses(f)
		if local == "" {
			if !fw.ignoreTCP && f.Protocol == "tcp" {
				logrus.Infof("Not forwarding TCP %s", remote)
			}
			if !fw.ignoreUDP && f.Protocol == "udp" {
				logrus.Infof("Not forwarding UDP %s", remote)
			}
			continue
		}
		logrus.Infof("Forwarding %s from %s to %s", strings.ToUpper(f.Protocol), remote, local)
		fw.closableListeners.Forward(ctx, client, f.Protocol, local, remote)
	}
	for _, f := range ev.LocalPortsRemoved {
		local, remote := fw.forwardingAddresses(f)
		if local == "" {
			continue
		}
		fw.closableListeners.Remove(ctx, f.Protocol, local, remote)
		logrus.Debugf("Port forwarding closed proto:%s host:%s guest:%s", f.Protocol, local, remote)
	}
}

func (fw *Forwarder) forwardingAddresses(guest *api.IPPort) (hostAddr, guestAddr string) {
	guestIP := net.ParseIP(guest.Ip)
	for _, rule := range fw.rules {
		if rule.GuestSocket != "" {
			continue
		}
		if guest.Port < int32(rule.GuestPortRange[0]) || guest.Port > int32(rule.GuestPortRange[1]) {
			continue
		}
		switch {
		case guestIP.IsUnspecified():
		case guestIP.Equal(rule.GuestIP):
		case guestIP.Equal(net.IPv6loopback) && rule.GuestIP.Equal(IPv4loopback1):
		case rule.GuestIP.IsUnspecified() && !rule.GuestIPMustBeZero:
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

func hostAddress(rule limayaml.PortForward, guest *api.IPPort) string {
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
