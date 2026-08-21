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
