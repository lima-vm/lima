// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package hostagent

import (
	"testing"

	"gotest.tools/v3/assert"

	"github.com/lima-vm/lima/v2/pkg/guestagent/api"
)

func TestValidateLocalPorts(t *testing.T) {
	ipPorts := []*api.IPPort{
		{Ip: "127.0.0.1", Port: 80, Protocol: "tcp"},
		// A compromised guest can report an arbitrary string instead of an IP;
		// this one carries ANSI escapes that would rewrite the operator's
		// terminal if it reached PortForwardEvent.GuestAddr.
		{Ip: "1.2.3.4\x1b[2Kspoofed\x07", Port: 8080, Protocol: "tcp"},
		{Ip: "::1", Port: 443, Protocol: "tcp"},
		{Ip: "not-an-ip", Port: 22, Protocol: "tcp"},
	}

	validated := validateLocalPorts(ipPorts)

	assert.Equal(t, len(validated), 2)
	assert.Equal(t, validated[0].Ip, "127.0.0.1")
	assert.Equal(t, validated[1].Ip, "::1")
}
