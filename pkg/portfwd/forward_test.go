// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package portfwd

import (
	"net"
	"testing"

	"gotest.tools/v3/assert"

	"github.com/lima-vm/lima/v2/pkg/guestagent/api"
	"github.com/lima-vm/lima/v2/pkg/limatype"
	"github.com/lima-vm/lima/v2/pkg/limayaml"
)

// testForwarder returns a forwarder configured to disable TCP forwarding, followed
// by the catch-all rule that the host agent always appends last.
func testForwarder() *Forwarder {
	rules := []limatype.PortForward{
		{GuestPortRange: [2]int{1, 65535}, Proto: limatype.ProtoTCP, Ignore: true},
		{},
	}
	for i := range rules {
		limayaml.FillPortForwardDefaults(&rules[i], "", limatype.User{}, nil)
	}
	return &Forwarder{rules: rules}
}

func TestForwardingAddressesRejectsUnknownProto(t *testing.T) {
	fw := testForwarder()
	for _, proto := range []string{"tcp6", "TCP", "any", ""} {
		hostAddr, _ := fw.forwardingAddresses(&api.IPPort{Ip: "127.0.0.1", Port: 8080, Protocol: proto})
		assert.Equal(t, hostAddr, "", "proto %#q", proto)
	}
}

func TestForwardingAddressesHonorsIgnoredProto(t *testing.T) {
	fw := testForwarder()
	tcpAddr, _ := fw.forwardingAddresses(&api.IPPort{Ip: "127.0.0.1", Port: 8080, Protocol: limatype.ProtoTCP})
	assert.Equal(t, tcpAddr, "")
	udpAddr, _ := fw.forwardingAddresses(&api.IPPort{Ip: "127.0.0.1", Port: 8080, Protocol: limatype.ProtoUDP})
	assert.Equal(t, udpAddr, "127.0.0.1:8080")
}

// ignoreAllForwarder returns a forwarder built from the catch-all rule that the documentation
// gives for disabling port forwarding, followed by the catch-all rule that the host agent always
// appends last.
func ignoreAllForwarder(guestIPMustBeZero *bool) *Forwarder {
	rules := []limatype.PortForward{
		{GuestIP: net.IPv4zero, GuestIPMustBeZero: guestIPMustBeZero, Proto: limatype.ProtoAny, Ignore: true},
		{},
	}
	for i := range rules {
		limayaml.FillPortForwardDefaults(&rules[i], "", limatype.User{}, nil)
	}
	return &Forwarder{rules: rules}
}

func TestForwardingAddressesIgnoresAllPorts(t *testing.T) {
	fw := ignoreAllForwarder(new(false))
	for _, ip := range []string{"0.0.0.0", "127.0.0.1", "::1", "192.168.5.15"} {
		for _, proto := range []string{limatype.ProtoTCP, limatype.ProtoUDP} {
			hostAddr, _ := fw.forwardingAddresses(&api.IPPort{Ip: ip, Port: 8080, Protocol: proto})
			assert.Equal(t, hostAddr, "", "guestIP %#q, proto %#q", ip, proto)
		}
	}
	// Leaving guestIPMustBeZero unset is not equivalent: it defaults to true when guestIP is
	// 0.0.0.0, so the rule no longer matches ports bound to a specific address in the guest.
	fw = ignoreAllForwarder(nil)
	hostAddr, _ := fw.forwardingAddresses(&api.IPPort{Ip: "127.0.0.1", Port: 8080, Protocol: limatype.ProtoTCP})
	assert.Equal(t, hostAddr, "127.0.0.1:8080")
}
