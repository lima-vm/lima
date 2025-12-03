// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package vmnet

import (
	"net"
	"testing"

	"gotest.tools/v3/assert"
)

// TestInterfaces tests that the Interfaces function correctly retrieves
// the list of network interfaces and matches the output of net.Interfaces.
func TestInterfaces(t *testing.T) {
	ifas, err := net.Interfaces()
	assert.NilError(t, err)
	assert.Assert(t, len(ifas) > 0)

	ifas2, err := NewInterfaces()
	assert.NilError(t, err)
	assert.Assert(t, len(ifas2) > 0)
	assert.Equal(t, len(ifas), len(ifas2))
	for i, ifa := range ifas {
		ifa2 := ifas2[i]
		assert.Equal(t, ifa.Index, ifa2.Index)
		assert.Equal(t, ifa.MTU, ifa2.MTU)
		assert.Equal(t, ifa.Name, ifa2.Name)
		assert.Equal(t, ifa.HardwareAddr.String(), ifa2.HardwareAddr.String())
		assert.Equal(t, ifa.Flags, ifa2.Flags)
		assert.Assert(t, ifa2.Type != 0)
	}
}
