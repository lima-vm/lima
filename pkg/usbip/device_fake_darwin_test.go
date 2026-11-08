// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

//go:build darwin && cgo && usbipfake

package usbip

import (
	"context"
	"errors"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"

	logrustest "github.com/sirupsen/logrus/hooks/test"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/poll"
)

var (
	setConfig1 = [8]byte{0x00, reqSetConfiguration, 1}
	getDesc    = [8]byte{0x80, 0x06, 0, 0x03, 0, 0, 255, 0}
)

func waitBlocked(t *testing.T) {
	t.Helper()
	poll.WaitOn(t, func(poll.LogT) poll.Result {
		if fakeStats().blocked > 0 {
			return poll.Success()
		}
		return poll.Continue("call not blocked yet")
	}, poll.WithDelay(time.Millisecond))
}

func setInterfaceSetup(num, alt byte) [8]byte {
	return [8]byte{0x01, reqSetInterface, alt, 0, num}
}

func TestZeroPacketReachesPipe(t *testing.T) {
	g := newFakeDevice(1)
	_, err := g.Control(t.Context(), setConfig1, nil)
	assert.NilError(t, err)
	assert.Equal(t, g.OutMaxPacket(2), 64)
	assert.Equal(t, g.OutMaxPacket(9), 0, "unknown endpoint")

	h := urbHeader{Direction: dirOut, Ep: 2, TransferFlags: urbZeroPacket, TransferBufferLength: 64}
	status, actual, _ := doSubmit(t.Context(), g, h, make([]byte, 64))
	assert.Equal(t, status, int32(0))
	assert.Equal(t, actual, int32(64))
	s := fakeStats()
	assert.Equal(t, int(s.writes), 2)
	assert.Equal(t, int(s.lastSize), 0)
	assert.Equal(t, int(s.lastBufNull), 1)

	// The terminator is a pipe call like any other, so an abort must free it.
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	fakeArmBlock(1)
	done := make(chan error, 1)
	go func() {
		_, err := g.Transfer(ctx, 2, false, nil)
		done <- err
	}()
	waitBlocked(t)
	cancel()
	assert.ErrorContains(t, <-done, "0xe00002eb")
	assert.NilError(t, g.Close())
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
	d, err := newDevice(svc, g.ptr, DeviceInfo{Busid: "1-0x01100000"})
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

func TestHeldDeviceHungControlEnds(t *testing.T) {
	d := heldDevice(t, 0, 1)
	c, done := startSession(t, d)
	fakeArmBlock(0)
	send(t, c, urbHeader{Command: cmdSubmit, Seqnum: 1, Direction: dirIn, TransferBufferLength: 255, Setup: getDesc})
	waitBlocked(t)

	assert.NilError(t, c.SetReadDeadline(time.Now().Add(5*time.Second)))
	send(t, c, urbHeader{Command: cmdUnlink, Seqnum: 2, TransferFlags: 1})
	r := unlinkReply(t, c, 1, 2, true)
	assert.Equal(t, int32(r.TransferFlags), int32(-errnoECONNRESET))

	s := fakeStats()
	assert.Assert(t, s.abortZeros > 0)
	bound := time.Duration(s.completionTimeout) * time.Millisecond
	assert.Assert(t, bound > 0 && bound < supersedeWait && bound < revokeWait, "bound %s", bound)

	assert.NilError(t, c.Close())
	assert.NilError(t, sessionEnd(t, done))
}

func TestIOReturnStatus(t *testing.T) {
	for _, tc := range []struct {
		ret    uint32
		want   int32
		clears int
	}{
		{0xe000404f, -errnoEPIPE, 0}, // a device stall is the guest's to clear
		{0xe0005000, -errnoEPIPE, 0},
		{0xe0004051, -errnoETIME, 1},
		{0xe00002d6, -errnoETIME, 1},
		{0xe00002e8, -errnoEOVERFLOW, 1},
		{0xe00002ed, -errnoEPROTO, 1},
		{0xe00002eb, -errnoEPROTO, 0},
		{0xe00002c5, -errnoEPROTO, 1},
		{0xe00002c0, -errnoENODEV, 0},
	} {
		g := newFakeDevice(1)
		_, err := g.Control(t.Context(), setConfig1, nil)
		assert.NilError(t, err)

		fakeSetRets(tc.ret, 0, 0, 0)
		status, _, _ := doSubmit(t.Context(), g, urbHeader{Direction: dirIn, Ep: 1}, nil)
		assert.Equal(t, status, tc.want, "bulk 0x%08x", tc.ret)
		assert.Equal(t, int(fakeStats().clears), tc.clears, "bulk 0x%08x", tc.ret)

		fakeSetRets(0, 0, 0, tc.ret)
		status, _, _ = doSubmit(t.Context(), g, urbHeader{Direction: dirIn, Setup: getDesc}, nil)
		assert.Equal(t, status, tc.want, "control 0x%08x", tc.ret)
	}
}

func TestReenumerateNeedsOpenDevice(t *testing.T) {
	old := reenumerateWait
	reenumerateWait = 0
	defer func() { reenumerateWait = old }()

	g := newFakeDevice(1)
	_, err := g.Control(t.Context(), setConfig1, nil)
	assert.NilError(t, err)
	g.reenumerate()
	assert.Equal(t, int(fakeStats().reenums), 1)
	assert.Equal(t, int(fakeStats().closes), 1, "interfaces close first")

	g = newFakeDevice(1)
	g.opened = false
	g.reenumerate()
	assert.Equal(t, int(fakeStats().reenums), 0)
}

func TestBlockingCallsMaskAsyncSignals(t *testing.T) {
	old := reenumerateWait
	reenumerateWait = 0
	defer func() { reenumerateWait = old }()

	control := func(setup [8]byte) func(*iokitDevice) error {
		return func(g *iokitDevice) error {
			_, err := g.Control(t.Context(), setup, nil)
			return err
		}
	}
	for name, call := range map[string]func(*iokitDevice) error{
		"read": func(g *iokitDevice) error {
			_, err := g.Transfer(t.Context(), 1, true, nil)
			return err
		},
		"write": func(g *iokitDevice) error {
			_, err := g.Transfer(t.Context(), 2, false, nil)
			return err
		},
		"control":           control(getDesc),
		"set configuration": control(setConfig1),
		"set interface":     control(setInterfaceSetup(0, 0)),
		"clear halt":        control([8]byte{0x02, reqClearFeature, 0, 0, 0x81}),
		"re-enumerate": func(g *iokitDevice) error {
			g.reenumerate()
			return nil
		},
	} {
		t.Run(name, func(t *testing.T) {
			g := newFakeDevice(1)
			_, err := g.Control(t.Context(), setConfig1, nil)
			assert.NilError(t, err)
			runtime.LockOSThread()
			defer runtime.UnlockOSThread()
			assert.Assert(t, !fakeThreadBlocks(syscall.SIGCHLD))
			assert.NilError(t, call(g))
			for _, sig := range []syscall.Signal{syscall.SIGCHLD, syscall.SIGURG, syscall.SIGPROF, syscall.SIGTERM} {
				assert.Assert(t, fakeIOBlocked(sig), "%v not blocked during call", sig)
			}
			for _, sig := range []syscall.Signal{syscall.SIGSEGV, syscall.SIGBUS, syscall.SIGFPE, syscall.SIGILL, syscall.SIGABRT, syscall.SIGTRAP} {
				assert.Assert(t, !fakeIOBlocked(sig), "%v blocked during call", sig)
			}
			assert.Assert(t, !fakeThreadBlocks(syscall.SIGCHLD), "mask not restored")
		})
	}
}

func TestUnlinkInFlightMissedAbort(t *testing.T) {
	g := newFakeDevice(1)
	c, done := startSession(t, g)
	fakeArmBlock(2)
	submit := urbHeader{Command: cmdSubmit, Seqnum: 1, Direction: dirIn, TransferBufferLength: 255, Setup: getDesc}
	send(t, c, submit)
	waitBlocked(t)

	assert.NilError(t, c.SetReadDeadline(time.Now().Add(5*time.Second)))
	send(t, c, urbHeader{Command: cmdUnlink, Seqnum: 2, TransferFlags: 1})
	r := unlinkReply(t, c, 1, 2, true)
	assert.Equal(t, int32(r.TransferFlags), int32(-errnoECONNRESET))
	assert.Assert(t, fakeStats().abortZeros >= 3)

	submit.Seqnum = 3
	send(t, c, submit)
	r, err := readURBHeader(c)
	assert.NilError(t, err)
	assert.Equal(t, r.Command, uint32(retSubmit))
	assert.Equal(t, r.Seqnum, uint32(3))

	assert.NilError(t, c.Close())
	assert.NilError(t, sessionEnd(t, done))
	assert.NilError(t, g.Close())
}

func TestPinDuringSetConfiguration(t *testing.T) {
	g := newFakeDevice(1)
	_, err := g.Control(t.Context(), setConfig1, nil)
	assert.NilError(t, err)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	fakeArmBlock(1 << 30)
	read := make(chan error, 1)
	go func() {
		_, err := g.Transfer(ctx, 1, true, make([]byte, 8))
		read <- err
	}()
	waitBlocked(t)
	cancel()

	_, err = g.Control(t.Context(), setConfig1, nil)
	assert.NilError(t, err)
	s := fakeStats()
	assert.Equal(t, int(s.closes), 1)
	assert.Equal(t, int(s.finalReleases), 0, "interface released under a running pipe call")

	before := s.aborts
	poll.WaitOn(t, func(poll.LogT) poll.Result {
		if fakeStats().aborts >= before+2 {
			return poll.Success()
		}
		return poll.Continue("abort loop not retrying")
	}, poll.WithDelay(5*time.Millisecond))
	assert.Equal(t, int(fakeStats().afterRelease), 0)

	fakeSetMissAborts(0)
	select {
	case err := <-read:
		assert.ErrorContains(t, err, "0xe00002eb")
	case <-time.After(5 * time.Second):
		assert.Assert(t, false, "read still blocked")
	}
	s = fakeStats()
	assert.Equal(t, int(s.afterRelease), 0)
	assert.Equal(t, int(s.finalReleases), 1)

	assert.NilError(t, g.Close())
	s = fakeStats()
	assert.Equal(t, int(s.afterRelease), 0)
	assert.Equal(t, int(s.finalReleases), 2)
}

func TestAbortJoinedBeforeRelease(t *testing.T) {
	g := newFakeDevice(1)
	_, err := g.Control(t.Context(), setConfig1, nil)
	assert.NilError(t, err)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	fakeArmBlock(1 << 30)
	read := make(chan error, 1)
	go func() {
		_, err := g.Transfer(ctx, 1, true, make([]byte, 8))
		read <- err
	}()
	waitBlocked(t)

	// The pin is now the only reference to the old interface.
	_, err = g.Control(t.Context(), setConfig1, nil)
	assert.NilError(t, err)
	const slow = 100 * time.Millisecond
	fakeSetSlowAbort(slow)
	cancel()
	poll.WaitOn(t, func(poll.LogT) poll.Result {
		if fakeStats().aborts > 0 {
			return poll.Success()
		}
		return poll.Continue("no abort yet")
	}, poll.WithDelay(time.Millisecond))
	fakeUnblock()
	assert.NilError(t, <-read)

	aborts := fakeStats().aborts
	time.Sleep(slow + 3*abortRetry)
	s := fakeStats()
	assert.Equal(t, int(s.afterRelease), 0, "abort still running after final release")
	assert.Equal(t, s.aborts, aborts, "abort loop outlived the call")
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
		BusNum: 1, DevNum: 1, Busid: "1-0x01100000",
		Vendor: 0x0403, Product: 0x6015, VendorName: "FTDI", ProductName: "FT231X USB UART",
	})

	nums := ft231x()
	nums["locationID"] = 0xfa120000
	delete(nums, "USB Address")
	info, ok = read(nums)
	assert.Assert(t, ok)
	assert.Equal(t, info.Busid, "250-0xfa120000")
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
	info := DeviceInfo{Busid: "1-0x01100000"}

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
