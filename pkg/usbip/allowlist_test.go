// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package usbip

import (
	"fmt"
	"sync"
	"testing"

	"gotest.tools/v3/assert"

	"github.com/lima-vm/lima/v2/pkg/limatype"
)

func TestWriteReadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	want := []AllowEntry{
		{Name: "alfa", VendorID: "0bda", ProductID: "8812", Busid: "20-3", RegistryID: 1<<63 + 1},
		{VendorID: "9ac4", ProductID: "4b8f", Denied: true},
	}
	assert.NilError(t, WriteAllowlist(dir, want))
	got, err := ReadAllowlist(dir)
	assert.NilError(t, err)
	assert.DeepEqual(t, got, want)
}

func TestAllowedMatching(t *testing.T) {
	list := []AllowEntry{{VendorID: "0BDA", ProductID: "8812"}}
	assert.Equal(t, Allowed(list, AllowEntry{VendorID: "0bda", ProductID: "8812"}), true)
	assert.Equal(t, Allowed(list, AllowEntry{VendorID: "1234", ProductID: "8812"}), false)
}

func TestUpdateAllowlistLosesNoEntry(t *testing.T) {
	dir := t.TempDir()
	var wg sync.WaitGroup
	for i := range 50 {
		wg.Go(func() {
			assert.Check(t, UpdateAllowlist(dir, func(list []AllowEntry) []AllowEntry {
				return AddEntry(list, AllowEntry{VendorID: fmt.Sprintf("%04x", i), ProductID: "0001"})
			}))
		})
	}
	wg.Wait()
	list, err := ReadAllowlist(dir)
	assert.NilError(t, err)
	assert.Equal(t, len(list), 50)
}

func TestGrantEndsWithPlug(t *testing.T) {
	plug := DeviceInfo{Vendor: 0x403, Product: 0x6015, Busid: "1-0x01100000", RegistryID: 0x100024570}
	replug := plug
	replug.RegistryID++

	grant := []AllowEntry{EntryFor(plug)}
	assert.Assert(t, Allowed(grant, EntryFor(plug)), "guest reboot re-imports the same plug")
	assert.Assert(t, !Allowed(grant, EntryFor(replug)), "replug must not reattach")

	assert.Assert(t, Allowed([]AllowEntry{{VendorID: "0403", ProductID: "6015"}}, EntryFor(replug)), "YAML wildcard")
	assert.Assert(t, Allowed([]AllowEntry{{VendorID: "0403", ProductID: "6015", BusAddr: plug.Busid}}, EntryFor(replug)), "YAML busAddr")

	attached := AddEntry(grant, EntryFor(replug))
	assert.Equal(t, len(attached), 1, "a new attach replaces the dead grant")
	assert.Assert(t, Allowed(attached, EntryFor(replug)))
	assert.Equal(t, len(RemoveEntry(grant, EntryFor(replug))), 0, "detach clears a dead grant")
}

func TestDeniedOverridesWildcard(t *testing.T) {
	wild := AllowEntry{VendorID: "0403", ProductID: "6015"}
	dev := AllowEntry{VendorID: "0403", ProductID: "6015", BusAddr: "1-1"}
	twin := AllowEntry{VendorID: "0403", ProductID: "6015", BusAddr: "1-2"}
	deny := AllowEntry{VendorID: "0403", ProductID: "6015", BusAddr: "1-1", Denied: true}

	assert.Equal(t, len(AddEntry([]AllowEntry{wild}, dev)), 2, "a grant keeps the YAML wildcard")
	for _, list := range [][]AllowEntry{{wild, deny}, {deny, wild}} {
		assert.Assert(t, !Allowed(list, dev))
		assert.Assert(t, Allowed(list, twin))
		assert.Assert(t, Allowed(AddEntry(list, dev), dev), "attach lifts the deny")
	}
}

func TestConfigEntries(t *testing.T) {
	got := ConfigEntries([]limatype.USBDevice{
		{Name: "a", VendorID: "0403", ProductID: "6015"},
		{VendorID: "0403", ProductID: "6015", BusAddr: "1-1"},
		{Name: "duplicate", VendorID: "0403", ProductID: "6015"},
	})
	assert.DeepEqual(t, got, []AllowEntry{
		{Name: "a", VendorID: "0403", ProductID: "6015"},
		{VendorID: "0403", ProductID: "6015", BusAddr: "1-1"},
	})
}
