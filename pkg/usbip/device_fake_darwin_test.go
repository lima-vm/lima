// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

//go:build darwin && cgo && usbipfake

package usbip

import (
	"errors"
	"strings"
	"testing"

	logrustest "github.com/sirupsen/logrus/hooks/test"
	"gotest.tools/v3/assert"
)

var (
	setConfig1 = [8]byte{0x00, reqSetConfiguration, 1}
	getDesc    = [8]byte{0x80, 0x06, 0, 0x03, 0, 0, 255, 0}
)

func setInterfaceSetup(num, alt byte) [8]byte {
	return [8]byte{0x01, reqSetInterface, alt, 0, num}
}

func TestDeconfigureReleasesInterfaces(t *testing.T) {
	g := newFakeDevice(2)
	_, err := g.Control(t.Context(), setConfig1, nil)
	assert.NilError(t, err)
	_, err = g.Control(t.Context(), [8]byte{0x00, reqSetConfiguration, 0}, nil)
	assert.NilError(t, err)
	assert.Equal(t, int(fakeStats().closes), 2)
	assert.Equal(t, len(g.pipes), 0)
	assert.NilError(t, g.Close())
}

func TestRoutingTwoInterfaces(t *testing.T) {
	g := newFakeDevice(2)
	_, err := g.Control(t.Context(), setConfig1, nil)
	assert.NilError(t, err)

	for range 200 {
		for _, tc := range []struct {
			ep   uint8
			in   bool
			intf int
			pipe int
		}{{1, true, 0, 1}, {2, false, 0, 2}, {3, true, 1, 1}, {4, false, 1, 2}} {
			_, err := g.Transfer(t.Context(), tc.ep, tc.in, make([]byte, 8))
			assert.NilError(t, err)
			s := fakeStats()
			assert.Equal(t, int(s.lastIntf), tc.intf)
			assert.Equal(t, int(s.lastPipe), tc.pipe)
			assert.Equal(t, int(s.lastBufNull), 0)
			assert.Equal(t, int(s.lastSize), 8)
		}
	}

	_, err = g.Transfer(t.Context(), 1, false, nil)
	assert.ErrorContains(t, err, "no interface provides endpoint")
	assert.Equal(t, int(fakeStats().opens), 2)
	assert.Equal(t, len(g.refused), 0, "rescan retried a held interface")

	assert.NilError(t, g.Close())
	assert.Equal(t, int(fakeStats().finalReleases), 2, "rescan leaked a plugin")
}

func TestLateNubClaimedOnMiss(t *testing.T) {
	g := newFakeDevice(2)
	fakeSetVisible(1)
	_, err := g.Control(t.Context(), setConfig1, nil)
	assert.NilError(t, err)
	assert.Equal(t, int(fakeStats().opens), 1)

	_, err = g.Transfer(t.Context(), 3, true, nil)
	assert.ErrorContains(t, err, "no interface provides endpoint")

	fakeSetVisible(2)
	_, err = g.Transfer(t.Context(), 3, true, nil)
	assert.NilError(t, err)
	s := fakeStats()
	assert.Equal(t, int(s.opens), 2)
	assert.Equal(t, int(s.lastIntf), 1)
}

func TestRefusedClaimWarnsOnce(t *testing.T) {
	const exclusive = 0xe00002c5
	hook := logrustest.NewGlobal()
	g := newFakeDevice(1)
	fakeSetRets(0, exclusive, 0, 0)
	_, err := g.Control(t.Context(), setConfig1, nil)
	assert.NilError(t, err, "a refused claim must not fail SET_CONFIGURATION")

	for range 3 {
		status, _, _ := doSubmit(t.Context(), g, urbHeader{Direction: dirIn, Ep: 1}, nil)
		assert.Equal(t, status, int32(-errnoEPIPE))
	}
	var warns []string
	for _, e := range hook.AllEntries() {
		if strings.Contains(e.Message, "claim interface") {
			warns = append(warns, e.Message)
		}
	}
	assert.Equal(t, len(warns), 1)
	assert.Assert(t, strings.Contains(warns[0], "claim interface 0: IOReturn 0xe00002c5"), warns[0])

	fakeSetRets(0, 0, 0, 0)
	status, _, _ := doSubmit(t.Context(), g, urbHeader{Direction: dirIn, Ep: 1}, nil)
	assert.Equal(t, status, int32(0))
}

func TestSetInterfaceReachesDevice(t *testing.T) {
	g := newFakeDevice(1)
	_, err := g.Control(t.Context(), setConfig1, nil)
	assert.NilError(t, err)

	for i, alt := range []byte{0, 1, 0, 0} {
		_, err := g.Control(t.Context(), setInterfaceSetup(0, alt), nil)
		assert.NilError(t, err)
		s := fakeStats()
		assert.Equal(t, int(s.altCalls), i+1)
		assert.Equal(t, int(s.lastAlt), int(alt))
		assert.Equal(t, int(s.opens), 1)
		assert.Equal(t, int(s.closes), 0)
	}

	_, err = g.Control(t.Context(), setInterfaceSetup(0, 1), nil)
	assert.NilError(t, err)
	_, err = g.Transfer(t.Context(), 1, true, nil)
	assert.ErrorContains(t, err, "no interface provides endpoint", "alt 1 has no 0x81")
	_, err = g.Transfer(t.Context(), 9, true, nil)
	assert.NilError(t, err)
}

func TestSetInterfaceFailureReopens(t *testing.T) {
	const stalled = 0xe000404f
	g := newFakeDevice(1)
	_, err := g.Control(t.Context(), setConfig1, nil)
	assert.NilError(t, err)

	fakeSetRets(0, 0, stalled, 0)
	status, _, _ := doSubmit(t.Context(), g, urbHeader{Direction: dirOut, Setup: setInterfaceSetup(0, 0)}, nil)
	assert.Equal(t, status, int32(-errnoEPIPE))
	assert.Equal(t, int(fakeStats().closes), 1)

	fakeSetRets(0, 0, 0, 0)
	_, err = g.Transfer(t.Context(), 1, true, nil)
	assert.NilError(t, err)
	assert.Equal(t, int(fakeStats().opens), 2)
}

func TestClearHalt(t *testing.T) {
	g := newFakeDevice(2)
	_, err := g.Control(t.Context(), setConfig1, nil)
	assert.NilError(t, err)

	for range 100 {
		_, err = g.Control(t.Context(), [8]byte{0x02, reqClearFeature, 0, 0, 0x83}, nil)
		assert.NilError(t, err)
	}
	s := fakeStats()
	assert.Equal(t, int(s.clears), 100)
	assert.Equal(t, int(s.requests), 0)
	assert.Equal(t, int(s.lastIntf), 1)
	assert.Equal(t, int(s.lastPipe), 1)

	_, err = g.Control(t.Context(), [8]byte{0x02, reqClearFeature, 0, 0, 0x8f}, nil)
	assert.NilError(t, err)
	s = fakeStats()
	assert.Equal(t, int(s.clears), 100, "unknown endpoint is forwarded")
	assert.Equal(t, int(s.requests), 1)
}

func TestControlActualLength(t *testing.T) {
	g := newFakeDevice(0)
	for _, tc := range []struct{ done, want int32 }{{4, 4}, {18, 18}, {0, 0}, {255, 255}, {300, 255}} {
		fakeSetCtrlDone(uint32(tc.done))
		h := urbHeader{Direction: dirIn, TransferBufferLength: 255, Setup: getDesc}
		status, actual, data := doSubmit(t.Context(), g, h, nil)
		assert.Equal(t, status, int32(0))
		assert.Equal(t, actual, tc.want)
		assert.Assert(t, len(data) >= int(actual))
	}
	s := fakeStats()
	assert.Equal(t, int(s.noDataTimeout), 0)
	assert.Equal(t, int(s.completionTimeout), 0)
}

// heldDevice gets opened=false from newDevice, as a real exclusive-access refusal does.
func heldDevice(t *testing.T, nubs, curConfig int) *iokitDevice {
	t.Helper()
	const exclusive = 0xe00002c5
	g, svc := newFakeDevice(nubs), fakeService()
	fakeSetDevice(1, 0, exclusive)
	fakeSetCurConfig(curConfig, 0)
	d, err := newDevice(svc, g.ptr, DeviceInfo{BusID: "1-0x01100000"})
	assert.NilError(t, err)
	assert.Assert(t, !d.opened)
	d.nubs, d.services = g.nubs, g.services
	t.Cleanup(func() { assert.NilError(t, d.Close()) })
	return d
}

func setConfigStatus(t *testing.T, g *iokitDevice, cfg byte) int32 {
	t.Helper()
	status, _, _ := doSubmit(t.Context(), g, urbHeader{Direction: dirOut, Setup: [8]byte{0x00, reqSetConfiguration, cfg}}, nil)
	return status
}

func TestHeldSetConfiguration(t *testing.T) {
	t.Run("other config refused, active config recovers", func(t *testing.T) {
		d := heldDevice(t, 1, 1)
		assert.Equal(t, setConfigStatus(t, d, 2), int32(-errnoEPIPE))
		assert.Equal(t, int(fakeStats().opens), 0, "interface of the wrong config claimed")
		assert.Equal(t, setConfigStatus(t, d, 1), int32(0))
		s := fakeStats()
		assert.Equal(t, int(s.opens), 1)
		assert.Equal(t, int(s.setConfigs), 0)
	})

	for name, tc := range map[string]struct {
		cur int
		cfg byte
	}{"holder left it unconfigured": {0, 1}, "deconfigure": {1, 0}} {
		t.Run(name, func(t *testing.T) {
			d := heldDevice(t, 1, tc.cur)
			assert.Equal(t, setConfigStatus(t, d, tc.cfg), int32(-errnoEPIPE))
			assert.Equal(t, int(fakeStats().opens), 0)
		})
	}

	t.Run("device gone", func(t *testing.T) {
		d := heldDevice(t, 1, 1)
		fakeSetCurConfig(1, 0xe00002c0)
		assert.Equal(t, setConfigStatus(t, d, 1), int32(-errnoENODEV))
	})
}

func TestSetConfigurationWithoutDriverMatching(t *testing.T) {
	g := newFakeDevice(1)
	var got []uint8
	g.reconfigure = func(num uint8) bool { got = append(got, num); return true }

	assert.Equal(t, setConfigStatus(t, g, 1), int32(0))
	assert.DeepEqual(t, got, []uint8{1})
	assert.Equal(t, int(fakeStats().setConfigs), 0, "IOUSBLib SetConfiguration lets macOS drivers match again")
	assert.Equal(t, int(fakeStats().opens), 1, "interfaces are claimed after the reconfigure")
	assert.NilError(t, g.Close())
}

func TestMountedVolumeSkipsSetConfiguration(t *testing.T) {
	g := newFakeDevice(1)
	fakeSetCurConfig(1, 0)
	mounted := true
	g.mounted = func() bool { return mounted }

	assert.Equal(t, setConfigStatus(t, g, 1), int32(0))
	assert.Equal(t, setConfigStatus(t, g, 2), int32(-errnoEPIPE))
	assert.Equal(t, int(fakeStats().setConfigs), 0)

	mounted = false
	assert.Equal(t, setConfigStatus(t, g, 1), int32(0))
	assert.Equal(t, int(fakeStats().setConfigs), 1, "an ejected disk takes the normal path")
	assert.NilError(t, g.Close())
}

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
