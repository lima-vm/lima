//go:build darwin && cgo

// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package usbip

import (
	"os"
	"path/filepath"
	"testing"

	"gotest.tools/v3/assert"
)

func TestUnpluggedKeepsOtherEntries(t *testing.T) {
	dir := t.TempDir()
	yaml := AllowEntry{VendorID: "0403", ProductID: "6015"}
	other := AllowEntry{VendorID: "1111", ProductID: "2222", BusAddr: "2-0x02100000", Busid: "2-0x02100000"}
	gone := AllowEntry{VendorID: "1111", ProductID: "2222", BusAddr: testBusid, Busid: testBusid}
	assert.NilError(t, WriteAllowlist(dir, []AllowEntry{yaml, gone, other}))

	NewProvider(dir).Unplugged(testBusid)

	list, err := ReadAllowlist(dir)
	assert.NilError(t, err)
	assert.DeepEqual(t, list, []AllowEntry{yaml, other})
}

// Reads the registry only. A zero ID would make a runtime grant survive unplug.
func TestListHasRegistryIDs(t *testing.T) {
	hosts, err := List()
	assert.NilError(t, err)
	for _, h := range hosts {
		assert.Assert(t, h.RegistryID != 0, h.Busid)
	}
}

// A guest picks the busid, so it must match a host device before it reaches a path.
func TestOpenUnknownBusidMakesNoLock(t *testing.T) {
	home := filepath.Join(t.TempDir(), "home")
	inst := filepath.Join(home, "vm1")
	assert.NilError(t, os.MkdirAll(inst, 0o755))
	t.Setenv("LIMA_HOME", home)
	assert.NilError(t, WriteAllowlist(inst, []AllowEntry{{VendorID: "0403", ProductID: "6015"}}))

	for _, busid := range []string{"9-0xdeadbeef", "/../../evil"} {
		path, err := lockPath(busid)
		assert.NilError(t, err)
		_, err = NewProvider(inst).Open(busid)
		assert.ErrorContains(t, err, "not found on host")
		_, err = os.Stat(path)
		assert.Assert(t, os.IsNotExist(err), path)
	}
}
