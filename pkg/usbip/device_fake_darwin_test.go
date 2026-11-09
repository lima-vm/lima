// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

//go:build darwin && cgo && usbipfake

package usbip

import (
	"errors"
	"testing"

	"gotest.tools/v3/assert"
)

func TestDeviceInfoFromRegistry(t *testing.T) {
	// From ioreg -p IOUSB -l on a real FT231X.
	ft231x := func() map[string]uint32 {
		return map[string]uint32{
			"idVendor": 1027, "idProduct": 24597, "bDeviceClass": 0, "locationID": 17825792, "USB Address": 1,
		}
	}
	read := func(nums map[string]uint32) (DeviceInfo, bool) {
		return deviceInfo(func(key string) (uint32, bool) {
			v, ok := nums[key]
			return v, ok
		}, func(key string) string {
			return map[string]string{"USB Vendor Name": "FTDI", "USB Product Name": "FT231X\x1b USB\n UART"}[key]
		})
	}

	info, ok := read(ft231x())
	assert.Assert(t, ok)
	assert.DeepEqual(t, info, DeviceInfo{
		BusNum: 1, DevNum: 1, BusID: "1-0x01100000",
		Vendor: 0x0403, Product: 0x6015, VendorName: "FTDI", ProductName: "FT231X USB UART",
	})

	nums := ft231x()
	nums["locationID"] = 0xfa120000
	delete(nums, "USB Address")
	info, ok = read(nums)
	assert.Assert(t, ok)
	assert.Equal(t, info.BusID, "250-0xfa120000")
	assert.Equal(t, info.BusNum, uint32(250))
	assert.Equal(t, info.DevNum, uint32(0))

	for class := range uint32(256) {
		nums := ft231x()
		nums["bDeviceClass"] = class
		_, ok := read(nums)
		assert.Equal(t, ok, class != 9, "class %d", class)
	}

	for key := range ft231x() {
		nums := ft231x()
		delete(nums, key)
		_, ok := read(nums)
		required := key == "locationID" || key == "idVendor" || key == "idProduct"
		assert.Equal(t, ok, !required, "missing %s", key)
	}
}

func TestGoneByRegistryIdentity(t *testing.T) {
	g := newFakeDevice(1)
	g.service = fakeService()
	other, replugged := fakeService(), fakeService()

	for name, tc := range map[string]struct {
		arm  func()
		want bool
	}{
		"still there":        {func() { g.fakeRegistry(nil, other, g.service) }, false},
		"replug in place":    {func() { g.fakeRegistry(nil, other, replugged) }, true},
		"unplugged":          {func() { g.fakeRegistry(nil, other) }, true},
		"no devices":         {func() { g.fakeRegistry(nil) }, true},
		"enumeration failed": {func() { g.fakeRegistry(errors.New("no port")) }, false},
	} {
		t.Run(name, func(t *testing.T) {
			tc.arm()
			assert.Equal(t, g.Gone(), tc.want)
			assert.Equal(t, fakeServiceRefs(g.service), 1, "lookup leaked")
			assert.Equal(t, fakeServiceRefs(other), 1, "lookup leaked")
			assert.Equal(t, fakeServiceRefs(replugged), 1, "lookup leaked")
		})
	}
}

func TestLinkSpeed(t *testing.T) {
	g := newFakeDevice(1)
	for speed, want := range map[int]uint32{0: 1, 1: 2, 2: 3, 3: 5, 4: 5, 5: 5} {
		fakeSetDevice(speed, 0, 0)
		got, err := linkSpeed(g.ptr)
		assert.NilError(t, err)
		assert.Equal(t, got, want, "IOKit speed %d", speed)
	}
}

func TestNewDevice(t *testing.T) {
	const (
		ioError   = 0xe00002bc
		exclusive = 0xe00002c5
	)
	info := DeviceInfo{BusID: "1-0x01100000"}

	t.Run("no speed fails import", func(t *testing.T) {
		g, svc := newFakeDevice(1), fakeService()
		fakeSetDevice(1, ioError, 0)
		_, err := newDevice(svc, g.ptr, info)
		assert.ErrorContains(t, err, "get device speed: IOReturn 0xe00002bc")
		assert.Equal(t, int(fakeStats().devOpens), 0)
		assert.Equal(t, fakeServiceRefs(svc), 1)
	})

	t.Run("open failure", func(t *testing.T) {
		g, svc := newFakeDevice(1), fakeService()
		fakeSetDevice(1, 0, ioError)
		_, err := newDevice(svc, g.ptr, info)
		assert.ErrorContains(t, err, "0xe00002bc")
		assert.Equal(t, fakeServiceRefs(svc), 1)
	})

	for name, tc := range map[string]struct {
		openRet uint32
		opened  bool
	}{"open": {0, true}, "held elsewhere": {exclusive, false}} {
		t.Run(name, func(t *testing.T) {
			g, svc := newFakeDevice(1), fakeService()
			fakeSetDevice(1, 0, tc.openRet)
			d, err := newDevice(svc, g.ptr, info)
			assert.NilError(t, err)
			assert.Equal(t, d.opened, tc.opened)
			assert.Equal(t, d.Info().Speed, uint32(usbipSpeedFull))
			assert.Equal(t, fakeServiceRefs(svc), 2, "device must keep its own reference")

			assert.NilError(t, d.Close())
			assert.Equal(t, fakeServiceRefs(svc), 1)
		})
	}
}
