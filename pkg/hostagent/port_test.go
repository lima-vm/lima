// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package hostagent

import (
	"testing"

	"gotest.tools/v3/assert"

	"github.com/lima-vm/lima/v2/pkg/guestagent/api"
	"github.com/lima-vm/lima/v2/pkg/limatype"
)

func testPortForwarder(sshLocalPort int) *portForwarder {
	return &portForwarder{
		rules: portForwardRules("", limatype.User{}, nil, sshLocalPort, nil),
	}
}

func TestPortForwardRulesBlockSSHOnEveryGuestIP(t *testing.T) {
	const sshLocalPort = 60022
	pf := testPortForwarder(sshLocalPort)
	for _, guestIP := range []string{"0.0.0.0", "127.0.0.1", "::", "::1"} {
		for _, port := range []int32{sshGuestPort, sshLocalPort} {
			hostAddr, _ := pf.forwardingAddresses(&api.IPPort{Ip: guestIP, Port: port, Protocol: "tcp"})
			assert.Equal(t, hostAddr, "", "guest %s:%d", guestIP, port)
		}
	}
}

func TestPortForwardRulesStillForwardOtherPorts(t *testing.T) {
	pf := testPortForwarder(60022)
	for _, guestIP := range []string{"0.0.0.0", "127.0.0.1", "::1"} {
		hostAddr, _ := pf.forwardingAddresses(&api.IPPort{Ip: guestIP, Port: 8080, Protocol: "tcp"})
		assert.Equal(t, hostAddr, "127.0.0.1:8080", "guest %s:8080", guestIP)
	}
}

func TestForwardingAddressesRejectsUntrustedGuestFields(t *testing.T) {
	pf := testPortForwarder(60022)
	// Ip and Protocol are proto3 strings supplied by the guest agent; a hostile
	// value must not be forwarded or returned as the guest address, otherwise the
	// bytes reach the host logs and the events limactl prints.
	for _, guest := range []*api.IPPort{
		{Ip: "127.0.0.1\x1b]0;pwned\x07", Port: 8080, Protocol: "tcp"},
		{Ip: "not-an-ip", Port: 8080, Protocol: "tcp"},
		{Ip: "127.0.0.1", Port: 8080, Protocol: "tcp\nforwarding evil"},
	} {
		hostAddr, guestAddr := pf.forwardingAddresses(guest)
		assert.Equal(t, hostAddr, "", "ip=%q proto=%q", guest.Ip, guest.Protocol)
		assert.Equal(t, guestAddr, "", "ip=%q proto=%q", guest.Ip, guest.Protocol)
	}
}
