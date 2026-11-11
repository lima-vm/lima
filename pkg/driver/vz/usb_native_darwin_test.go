//go:build darwin && !no_vz

// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package vz

import (
	"os/exec"
	"regexp"
	"strconv"
	"testing"

	"gotest.tools/v3/assert"
)

// TestLookupUSBRegistryInfo compares lookupUSBRegistryInfo with ioreg.
func TestLookupUSBRegistryInfo(t *testing.T) {
	out, err := exec.CommandContext(t.Context(), "ioreg", "-p", "IOUSB", "-w0").Output()
	assert.NilError(t, err)
	// +-o bladeRF 2.0@00210000  <class IOUSBHostDevice, id 0x100005cab, ...
	matches := regexp.MustCompile(`-o .+@([0-9a-f]+)\s+<class IOUSBHostDevice, id (0x[0-9a-f]+),`).FindAllStringSubmatch(string(out), -1)
	if len(matches) == 0 {
		t.Skip("no USB device is connected")
	}
	for _, m := range matches {
		location, err := strconv.ParseUint(m[1], 16, 32)
		assert.NilError(t, err)
		id, err := strconv.ParseUint(m[2], 0, 64)
		assert.NilError(t, err)
		info, ok := lookupUSBRegistryInfo(id)
		assert.Assert(t, ok, "device %#x not found", id)
		assert.Equal(t, info.LocationID, uint32(location), "device %#x", id)
	}

	_, ok := lookupUSBRegistryInfo(0xffffffffffff)
	assert.Assert(t, !ok)
}
