// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package portfwd

import (
	"testing"

	"gotest.tools/v3/assert"

	"github.com/lima-vm/lima/v2/pkg/guestagent/api"
	"github.com/lima-vm/lima/v2/pkg/limatype"
	"github.com/lima-vm/lima/v2/pkg/limayaml"
)

func testForwarder() *Forwarder {
	rule := limatype.PortForward{}
	limayaml.FillPortForwardDefaults(&rule, "", limatype.User{}, nil)
	return NewPortForwarder([]limatype.PortForward{rule}, false, false, nil)
}

func TestForwardingAddressesForwardsValidPort(t *testing.T) {
	fw := testForwarder()
	hostAddr, _ := fw.forwardingAddresses(&api.IPPort{Ip: "127.0.0.1", Port: 8080, Protocol: "tcp"})
	assert.Equal(t, hostAddr, "127.0.0.1:8080")
}

func TestForwardingAddressesRejectsUntrustedGuestFields(t *testing.T) {
	fw := testForwarder()
	// Ip and Protocol are proto3 strings supplied by the guest agent; a hostile
	// value must not be forwarded or returned as the guest address, otherwise the
	// bytes reach the host logs and the events limactl prints.
	for _, guest := range []*api.IPPort{
		{Ip: "127.0.0.1\x1b]0;pwned\x07", Port: 8080, Protocol: "tcp"},
		{Ip: "not-an-ip", Port: 8080, Protocol: "tcp"},
		{Ip: "127.0.0.1", Port: 8080, Protocol: "tcp\nforwarding evil"},
	} {
		hostAddr, guestAddr := fw.forwardingAddresses(guest)
		assert.Equal(t, hostAddr, "", "ip=%q proto=%q", guest.Ip, guest.Protocol)
		assert.Equal(t, guestAddr, "", "ip=%q proto=%q", guest.Ip, guest.Protocol)
	}
}
