//go:build darwin && !no_vz

// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package vz

import "testing"

func TestVZSlotSet(t *testing.T) {
	s := newSlotSet(vzHotMountSlots)
	seen := map[int]bool{}
	for i := 0; i < vzHotMountSlots; i++ {
		slot, ok := s.allocate()
		if !ok {
			t.Fatalf("allocation %d should succeed", i)
		}
		if seen[slot] {
			t.Fatalf("slot %d allocated twice", slot)
		}
		seen[slot] = true
	}
	if _, ok := s.allocate(); ok {
		t.Errorf("allocation past capacity should fail")
	}
	s.release(2)
	slot, ok := s.allocate()
	if !ok || slot != 2 {
		t.Errorf("expected to reuse freed slot 2, got slot=%d ok=%v", slot, ok)
	}
}

func TestHotMountTag(t *testing.T) {
	if got := hotMountTag(0); got != "lima-hotmount-0" {
		t.Errorf("hotMountTag(0)=%q", got)
	}
	if got := hotMountTag(7); got != "lima-hotmount-7" {
		t.Errorf("hotMountTag(7)=%q", got)
	}
}
