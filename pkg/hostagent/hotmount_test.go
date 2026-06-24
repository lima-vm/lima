// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package hostagent

import (
	"context"
	"fmt"
	"testing"

	"gotest.tools/v3/assert"

	"github.com/lima-vm/lima/v2/pkg/driver"
	"github.com/lima-vm/lima/v2/pkg/limatype"
	"github.com/lima-vm/lima/v2/pkg/ptr"
)

// fakeHotPlugDriver implements driver.Driver (via the embedded nil interface, whose
// methods are never called here) and driver.FSHotPlugger.
type fakeHotPlugDriver struct {
	driver.Driver
	plugged   []*driver.HotPlugFSRequest
	unplugged []string
	nextID    int
}

func (f *fakeHotPlugDriver) HotPlugFS(_ context.Context, req *driver.HotPlugFSRequest) (*driver.HotPlugFSResponse, error) {
	f.plugged = append(f.plugged, req)
	f.nextID++
	return &driver.HotPlugFSResponse{DeviceID: fmt.Sprintf("dev-%d", f.nextID)}, nil
}

func (f *fakeHotPlugDriver) HotUnplugFS(_ context.Context, req *driver.HotUnplugFSRequest) error {
	f.unplugged = append(f.unplugged, req.DeviceID)
	return nil
}

// nonHotPlugDriver implements driver.Driver but NOT driver.FSHotPlugger.
type nonHotPlugDriver struct{ driver.Driver }

func newTestAgent(d driver.Driver) (*HostAgent, *[]string) {
	execs := new([]string)
	a := &HostAgent{
		driver:     d,
		hotMounts:  make(map[string]*activeMount),
		instConfig: &limatype.LimaYAML{OS: ptr.Of(limatype.LINUX)},
	}
	a.guestExec = func(_, description string) (string, string, error) {
		*execs = append(*execs, description)
		return "", "", nil
	}
	return a, execs
}

func TestMountAddRemoveVirtiofs(t *testing.T) {
	fake := &fakeHotPlugDriver{}
	a, execs := newTestAgent(fake)
	hostDir := t.TempDir()

	m, err := a.MountAdd(context.Background(), hostDir, "/mnt/code", limatype.VIRTIOFS, true)
	assert.NilError(t, err)
	assert.Equal(t, m.Type, "virtiofs")
	assert.Equal(t, m.MountPoint, "/mnt/code")
	assert.Equal(t, len(fake.plugged), 1)
	assert.Equal(t, fake.plugged[0].Type, limatype.VIRTIOFS)
	assert.Equal(t, len(*execs), 1) // guest mount command ran
	assert.Equal(t, len(a.MountList()), 1)

	_, err = a.MountAdd(context.Background(), hostDir, "/mnt/code", limatype.VIRTIOFS, true)
	assert.ErrorContains(t, err, "already mounted")

	assert.NilError(t, a.MountRemove(context.Background(), "/mnt/code"))
	assert.Equal(t, len(fake.unplugged), 1)
	assert.Equal(t, len(a.MountList()), 0)
}

func TestMountAdd9pSetsNinePReq(t *testing.T) {
	fake := &fakeHotPlugDriver{}
	a, _ := newTestAgent(fake)
	hostDir := t.TempDir()

	_, err := a.MountAdd(context.Background(), hostDir, "/mnt/9p", limatype.NINEP, false)
	assert.NilError(t, err)
	assert.Equal(t, fake.plugged[0].Type, limatype.NINEP)
	assert.Assert(t, fake.plugged[0].NineP != nil)
}

func TestMountAddDefaultsToVirtiofs(t *testing.T) {
	fake := &fakeHotPlugDriver{}
	a, _ := newTestAgent(fake)
	hostDir := t.TempDir()

	m, err := a.MountAdd(context.Background(), hostDir, "/mnt/x", "", true)
	assert.NilError(t, err)
	assert.Equal(t, m.Type, "virtiofs")
}

func TestMountAddUnsupportedDriver(t *testing.T) {
	a, _ := newTestAgent(&nonHotPlugDriver{})
	hostDir := t.TempDir()

	_, err := a.MountAdd(context.Background(), hostDir, "/mnt/x", limatype.VIRTIOFS, true)
	assert.ErrorContains(t, err, "hot-plug")
}

func TestMountAddRejectsReservedAndMissing(t *testing.T) {
	a, _ := newTestAgent(&fakeHotPlugDriver{})
	_, err := a.MountAdd(context.Background(), t.TempDir(), "/etc", limatype.VIRTIOFS, true)
	assert.ErrorContains(t, err, "reserved")

	_, err = a.MountAdd(context.Background(), "/no/such/host/dir", "/mnt/x", limatype.VIRTIOFS, true)
	assert.ErrorContains(t, err, "host path")
}

func TestMountRemoveUnknown(t *testing.T) {
	a, _ := newTestAgent(&fakeHotPlugDriver{})
	err := a.MountRemove(context.Background(), "/mnt/nope")
	assert.ErrorContains(t, err, "not a hot-mount")
}
