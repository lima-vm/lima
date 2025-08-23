// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package usernet

import (
	"net"
	"testing"

	"gotest.tools/v3/assert"

	"github.com/lima-vm/lima/v2/pkg/networks"
)

func TestUsernetConfig(t *testing.T) {
	t.Run("verify dns ip", func(t *testing.T) {
		subnet, _, err := net.ParseCIDR(networks.SlirpNetwork)
		assert.NilError(t, err)
		assert.Equal(t, DNSIP(subnet), "192.168.5.3")
	})

	t.Run("verify gateway ip", func(t *testing.T) {
		subnet, _, err := net.ParseCIDR(networks.SlirpNetwork)
		assert.NilError(t, err)
		assert.Equal(t, GatewayIP(subnet), "192.168.5.2")
	})

	t.Run("verify subnet via config ip", func(t *testing.T) {
		subnet, err := Subnet("user-v2")
		assert.NilError(t, err)
		assert.Equal(t, subnet.String(), "192.168.104.0")
	})
}
