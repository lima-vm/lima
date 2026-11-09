// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package usbip

import (
	"fmt"
	"sync"
	"testing"

	"gotest.tools/v3/assert"
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
