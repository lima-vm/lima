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

// lockProvider opens a fake device under the busid lock.
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
