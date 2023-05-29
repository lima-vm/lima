package networks

import (
	"net"
	"testing"

	"gotest.tools/v3/assert"
)

func TestFillDefault(t *testing.T) {
	nwYaml := YAML{}
	newYaml, err := fillDefaults(nwYaml)
	assert.Check(t, err == nil)

	userNet := newYaml.Networks[ModeUserV2]
	assert.Equal(t, userNet.Mode, ModeUserV2)
	assert.Equal(t, userNet.Interface, "")
	assert.DeepEqual(t, userNet.NetMask, net.IP{})
	assert.DeepEqual(t, userNet.Gateway, net.IP{})
	assert.DeepEqual(t, userNet.DHCPEnd, net.IP{})
}
