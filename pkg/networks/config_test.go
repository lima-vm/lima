package networks

import (
	"net"
	"testing"

	"gotest.tools/v3/assert"
)

func TestFillDefault(t *testing.T) {
	nwYaml := YAML{}
	newYaml, err := fillDefaults(nwYaml)
	assert.NilError(t, err)

	userNet := newYaml.Networks[ModeUserV2]
	assert.Equal(t, userNet.Mode, ModeUserV2)
	assert.Equal(t, userNet.Interface, "")
	assert.DeepEqual(t, userNet.NetMask, net.ParseIP("255.255.255.0"))
	assert.DeepEqual(t, userNet.Gateway, net.ParseIP("192.168.104.1"))
	assert.DeepEqual(t, userNet.DHCPEnd, net.IP{})
}

func TestFillDefaultWithV2(t *testing.T) {
	nwYaml := YAML{Networks: map[string]Network{
		"user-v2": {Mode: ModeUserV2},
	}}
	newYaml, err := fillDefaults(nwYaml)
	assert.NilError(t, err)

	userNet := newYaml.Networks[ModeUserV2]
	assert.Equal(t, userNet.Mode, ModeUserV2)
	assert.Equal(t, userNet.Interface, "")
	assert.DeepEqual(t, userNet.NetMask, net.ParseIP("255.255.255.0"))
	assert.DeepEqual(t, userNet.Gateway, net.ParseIP("192.168.104.1"))
	assert.DeepEqual(t, userNet.DHCPEnd, net.IP{})
}

func TestFillDefaultWithV2AndGateway(t *testing.T) {
	nwYaml := YAML{Networks: map[string]Network{
		"user-v2": {Mode: ModeUserV2, Gateway: net.ParseIP("192.168.105.1")},
	}}
	newYaml, err := fillDefaults(nwYaml)
	assert.NilError(t, err)

	userNet := newYaml.Networks[ModeUserV2]
	assert.Equal(t, userNet.Mode, ModeUserV2)
	assert.Equal(t, userNet.Interface, "")
	assert.DeepEqual(t, userNet.NetMask, net.IP{})
	assert.DeepEqual(t, userNet.Gateway, net.ParseIP("192.168.105.1"))
	assert.DeepEqual(t, userNet.DHCPEnd, net.IP{})
}
