//go:build darwin && !no_vz

// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package vz

import (
	"testing"

	"gotest.tools/v3/assert"
)

func TestVZSlotSet(t *testing.T) {
	s := newSlotSet(vzHotMountSlots)
	seen := map[int]bool{}
	for i := range vzHotMountSlots {
		slot, ok := s.allocate()
		assert.Assert(t, ok, "allocation %d should succeed", i)
		assert.Assert(t, !seen[slot], "slot %d allocated twice", slot)
		seen[slot] = true
	}
	_, ok := s.allocate()
	assert.Assert(t, !ok, "allocation past capacity should fail")
	s.release(2)
	slot, ok := s.allocate()
	assert.Assert(t, ok)
	assert.Equal(t, slot, 2)
}

func TestHotMountTag(t *testing.T) {
	assert.Equal(t, hotMountTag(0), "lima-hotmount-0")
	assert.Equal(t, hotMountTag(7), "lima-hotmount-7")
}
