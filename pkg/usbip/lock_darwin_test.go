// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package usbip

import (
	"os"
	"strings"
	"testing"
	"time"

	logrustest "github.com/sirupsen/logrus/hooks/test"
	"golang.org/x/sys/unix"
	"gotest.tools/v3/assert"
)

const testBusid = "1-0x01100000"

// lockProvider opens as the real provider does, with a fake device under the lock.
type lockProvider struct {
	testProvider
	holder string
}

func (p *lockProvider) Open(busid string) (Device, error) {
	lock, err := lockBusid(busid, p.holder)
	if err != nil {
		return nil, err
	}
	return &lockedDevice{Device: fakeDev{}, lock: lock, holder: p.holder, reenumerate: func() {}}, nil
}

func TestSecondVMRefusedByName(t *testing.T) {
	t.Setenv("LIMA_HOME", t.TempDir())
	hook := logrustest.NewGlobal()
	vm1 := NewServer(&lockProvider{holder: "vm1"})
	vm2 := NewServer(&lockProvider{holder: "vm2"})

	c1, status, done1 := importBusid(t, vm1, testBusid)
	assert.Equal(t, status, uint32(0))
	name, held := Holder(testBusid)
	assert.Assert(t, held)
	assert.Equal(t, name, "vm1")

	_, status, _ = importBusid(t, vm2, testBusid)
	assert.Equal(t, status, uint32(1))
	refusal, err := hook.LastEntry().String()
	assert.NilError(t, err)
	assert.Assert(t, strings.Contains(refusal, `held by instance`) && strings.Contains(refusal, "vm1"), refusal)

	assert.NilError(t, c1.Close())
	_ = sessionEnd(t, done1)
	_, status, _ = importBusid(t, vm2, testBusid)
	assert.Equal(t, status, uint32(0))
}

func TestHolderOfFreeFile(t *testing.T) {
	t.Setenv("LIMA_HOME", t.TempDir())
	_, held := Holder(testBusid)
	assert.Assert(t, !held, "no file")

	f, err := lockBusid(testBusid, "crashed")
	assert.NilError(t, err)
	assert.NilError(t, f.Close())
	name, held := Holder(testBusid)
	assert.Assert(t, !held, "stale name %q", name)

	f, err = lockBusid(testBusid, "vm1")
	assert.NilError(t, err)
	defer f.Close()
	name, _ = Holder(testBusid)
	assert.Equal(t, name, "vm1", "old holder name not replaced")
}

func TestLockSurvivesHolderProbe(t *testing.T) {
	t.Setenv("LIMA_HOME", t.TempDir())
	path, err := lockPath(testBusid)
	assert.NilError(t, err)
	probe, err := os.OpenFile(path, os.O_CREATE|os.O_RDONLY, 0o600)
	assert.NilError(t, err)
	assert.NilError(t, unix.Flock(int(probe.Fd()), unix.LOCK_SH|unix.LOCK_NB))
	time.AfterFunc(30*time.Millisecond, func() { _ = probe.Close() })

	f, err := lockBusid(testBusid, "vm1")
	assert.NilError(t, err, "a probe must not refuse an import")
	assert.NilError(t, f.Close())
}

func TestRevokeLeavesOtherHolder(t *testing.T) {
	t.Setenv("LIMA_HOME", t.TempDir())
	assert.NilError(t, Revoke(testBusid, "vm1"), "not held")

	f, err := lockBusid(testBusid, "vm2")
	assert.NilError(t, err)
	defer f.Close()
	assert.NilError(t, Revoke(testBusid, "vm1"))
	name, held := Holder(testBusid)
	assert.Assert(t, held)
	assert.Equal(t, name, "vm2")
}

func TestRevokeWaitIsBounded(t *testing.T) {
	t.Setenv("LIMA_HOME", t.TempDir())
	revokeWait = 100 * time.Millisecond
	t.Cleanup(func() { revokeWait = 10 * time.Second })
	f, err := lockBusid(testBusid, "vm1")
	assert.NilError(t, err)
	defer f.Close()
	assert.ErrorContains(t, Revoke(testBusid, "vm1"), "still held")
}

func TestLockGrantedSeesDetach(t *testing.T) {
	t.Setenv("LIMA_HOME", t.TempDir())
	dir := t.TempDir()
	info := DeviceInfo{Busid: testBusid, Vendor: 0x403, Product: 0x6015}
	_, err := lockGranted(dir, "vm1", info)
	assert.ErrorContains(t, err, "not permitted")
	_, held := Holder(testBusid)
	assert.Assert(t, !held, "refused import kept the lock")

	assert.NilError(t, WriteAllowlist(dir, []AllowEntry{EntryFor(info)}))
	f, err := lockGranted(dir, "vm1", info)
	assert.NilError(t, err)
	assert.NilError(t, f.Close())
}

func TestLockedDeviceGone(t *testing.T) {
	t.Setenv("LIMA_HOME", t.TempDir())
	f, err := lockBusid(testBusid, "vm1")
	assert.NilError(t, err)
	d := &lockedDevice{Device: goneDev{gone: func() bool { return false }}, lock: f, holder: "vm1", reenumerate: func() {}}
	defer d.Close()
	assert.Assert(t, !d.Gone())

	path, err := lockPath(testBusid)
	assert.NilError(t, err)
	assert.NilError(t, os.Truncate(path, 0))
	assert.Assert(t, d.Gone())

	d.Device = goneDev{gone: func() bool { return true }}
	_, err = f.WriteAt([]byte("vm1"), 0)
	assert.NilError(t, err)
	assert.Assert(t, d.Gone(), "a real unplug still counts")
}

// heldAtClose records whether the lock still stood when the device closed.
type heldAtClose struct {
	fakeDev
	held *bool
}

func (d heldAtClose) Close() error {
	_, *d.held = Holder(testBusid)
	return nil
}

func TestLockedDeviceClose(t *testing.T) {
	t.Setenv("LIMA_HOME", t.TempDir())
	for _, detach := range []bool{false, true} {
		f, err := lockBusid(testBusid, "vm1")
		assert.NilError(t, err)
		calls, held := 0, false
		d := &lockedDevice{Device: heldAtClose{held: &held}, lock: f, holder: "vm1", reenumerate: func() { calls++ }}
		if detach {
			assert.NilError(t, f.Truncate(0))
		}
		assert.NilError(t, d.Close())
		assert.Equal(t, calls == 1, detach, "re-enumerate on detach only")
		assert.Assert(t, held, "next holder could open a device still in use")
		_, held = Holder(testBusid)
		assert.Assert(t, !held)
	}
}
