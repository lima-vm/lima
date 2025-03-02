/*
Copyright The Lima Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package usernet

import (
	"net"
	"testing"

	"github.com/lima-vm/lima/pkg/networks"
	"gotest.tools/v3/assert"
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
