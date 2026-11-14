// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"testing"

	"gotest.tools/v3/assert"

	"github.com/lima-vm/lima/v2/pkg/driver"
)

func TestResolveUSBDevice(t *testing.T) {
	devs := []driver.USBDevice{
		{RegistryID: 0x100, VendorID: 0x05ac, ProductID: 0x12a8},
		{RegistryID: 0x200, VendorID: 0x1234, ProductID: 0x5678},
		{RegistryID: 0x300, VendorID: 0x1234, ProductID: 0x5678},
	}

	dev, err := resolveUSBDevice(devs, "05ac:12a8", true)
	assert.NilError(t, err)
	assert.Equal(t, dev.RegistryID, uint64(0x100))

	dev, err = resolveUSBDevice(devs, "0x05AC:0x12A8", true)
	assert.NilError(t, err)
	assert.Equal(t, dev.RegistryID, uint64(0x100))

	dev, err = resolveUSBDevice(devs, "0x300", true)
	assert.NilError(t, err)
	assert.Equal(t, dev.RegistryID, uint64(0x300))

	dev, err = resolveUSBDevice(devs, "512", true)
	assert.NilError(t, err)
	assert.Equal(t, dev.RegistryID, uint64(0x200))

	_, err = resolveUSBDevice(devs, "1234:5678", true)
	assert.ErrorContains(t, err, "2 USB devices match")

	// Of several matching devices, only one is attached.
	devs[2].Attached = true
	dev, err = resolveUSBDevice(devs, "1234:5678", true)
	assert.NilError(t, err)
	assert.Equal(t, dev.RegistryID, uint64(0x200))
	dev, err = resolveUSBDevice(devs, "1234:5678", false)
	assert.NilError(t, err)
	assert.Equal(t, dev.RegistryID, uint64(0x300))
	devs[1].Attached = true
	_, err = resolveUSBDevice(devs, "1234:5678", true)
	assert.ErrorContains(t, err, "2 USB devices match")
	devs[1].Attached, devs[2].Attached = false, false

	_, err = resolveUSBDevice(devs, "dead:beef", true)
	assert.ErrorContains(t, err, `no USB device "dead:beef" found`)

	_, err = resolveUSBDevice(devs, "0x999", true)
	assert.ErrorContains(t, err, "no USB device with registry ID 0x999")

	_, err = resolveUSBDevice(devs, "05ac:zzzz", true)
	assert.ErrorContains(t, err, "invalid product ID")

	_, err = resolveUSBDevice(devs, "keyboard", true)
	assert.ErrorContains(t, err, "VENDOR:PRODUCT")
}
