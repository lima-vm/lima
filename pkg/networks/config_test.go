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

package networks

import (
	"net"
	"testing"

	"gotest.tools/v3/assert"
)

func TestFillDefault(t *testing.T) {
	cfg, err := fillDefaults(Config{})
	assert.NilError(t, err)

	userNet := cfg.Networks[ModeUserV2]
	assert.Equal(t, userNet.Mode, ModeUserV2)
	assert.Equal(t, userNet.Interface, "")
	assert.DeepEqual(t, userNet.NetMask, net.ParseIP("255.255.255.0"))
	assert.DeepEqual(t, userNet.Gateway, net.ParseIP("192.168.104.1"))
	assert.DeepEqual(t, userNet.DHCPEnd, net.IP{})
}

func TestFillDefaultWithV2(t *testing.T) {
	cfg := Config{Networks: map[string]Network{
		"user-v2": {Mode: ModeUserV2},
	}}
	cfg, err := fillDefaults(cfg)
	assert.NilError(t, err)

	userNet := cfg.Networks[ModeUserV2]
	assert.Equal(t, userNet.Mode, ModeUserV2)
	assert.Equal(t, userNet.Interface, "")
	assert.DeepEqual(t, userNet.NetMask, net.ParseIP("255.255.255.0"))
	assert.DeepEqual(t, userNet.Gateway, net.ParseIP("192.168.104.1"))
	assert.DeepEqual(t, userNet.DHCPEnd, net.IP{})
}

func TestFillDefaultWithV2AndGateway(t *testing.T) {
	cfg := Config{Networks: map[string]Network{
		"user-v2": {Mode: ModeUserV2, Gateway: net.ParseIP("192.168.105.1")},
	}}
	cfg, err := fillDefaults(cfg)
	assert.NilError(t, err)

	userNet := cfg.Networks[ModeUserV2]
	assert.Equal(t, userNet.Mode, ModeUserV2)
	assert.Equal(t, userNet.Interface, "")
	assert.DeepEqual(t, userNet.NetMask, net.IP{})
	assert.DeepEqual(t, userNet.Gateway, net.ParseIP("192.168.105.1"))
	assert.DeepEqual(t, userNet.DHCPEnd, net.IP{})
}
