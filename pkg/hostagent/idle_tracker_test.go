// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package hostagent

import (
	"testing"
	"time"

	"gotest.tools/v3/assert"
)

func TestIdleTracker_InitiallyNotIdle(t *testing.T) {
	tracker := NewIdleTracker(5 * time.Minute)
	assert.Assert(t, !tracker.IsIdle(), "tracker should not be idle immediately after creation")
}

func TestIdleTracker_BecomesIdle(t *testing.T) {
	tracker := NewIdleTracker(50 * time.Millisecond)
	time.Sleep(60 * time.Millisecond)
	assert.Assert(t, tracker.IsIdle(), "tracker should be idle after timeout")
}

func TestIdleTracker_TouchResetsIdle(t *testing.T) {
	tracker := NewIdleTracker(50 * time.Millisecond)
	time.Sleep(30 * time.Millisecond)
	tracker.Touch()
	time.Sleep(30 * time.Millisecond)
	assert.Assert(t, !tracker.IsIdle(), "tracker should not be idle after Touch()")
}

func TestIdleTracker_IdleDuration(t *testing.T) {
	tracker := NewIdleTracker(5 * time.Minute)
	time.Sleep(20 * time.Millisecond)
	dur := tracker.IdleDuration()
	assert.Assert(t, dur >= 15*time.Millisecond, "IdleDuration should report elapsed time since last activity, got %v", dur)
}

func TestIdleTracker_ConcurrentAccess(_ *testing.T) {
	tracker := NewIdleTracker(1 * time.Second)
	done := make(chan struct{})
	go func() {
		for range 100 {
			tracker.Touch()
		}
		close(done)
	}()
	for range 100 {
		_ = tracker.IsIdle()
		_ = tracker.IdleDuration()
	}
	<-done
}

// --- Edge case tests ---

func TestIdleTracker_ZeroTimeout(t *testing.T) {
	// Zero timeout means immediately idle.
	tracker := NewIdleTracker(0)
	assert.Assert(t, tracker.IsIdle(), "zero timeout should be immediately idle")
}

func TestIdleTracker_VeryLargeTimeout(t *testing.T) {
	tracker := NewIdleTracker(24 * time.Hour)
	assert.Assert(t, !tracker.IsIdle(), "24h timeout should not be immediately idle")
	assert.Assert(t, tracker.IdleDuration() < time.Second)
}

func TestIdleTracker_TouchThenImmediateCheck(t *testing.T) {
	tracker := NewIdleTracker(1 * time.Hour)
	tracker.Touch()
	assert.Assert(t, !tracker.IsIdle(), "should not be idle right after Touch")
	assert.Assert(t, tracker.IdleDuration() < 10*time.Millisecond)
}

// --- BusyCheck tests ---

func TestIdleTracker_NoBusyChecks_BackwardCompatible(t *testing.T) {
	// Zero busy-checks should behave exactly like the original IdleTracker.
	tracker := NewIdleTracker(50 * time.Millisecond)
	assert.Assert(t, !tracker.IsIdle(), "should not be idle initially")
	time.Sleep(60 * time.Millisecond)
	assert.Assert(t, tracker.IsIdle(), "should be idle after timeout with no busy-checks")
}

func TestIdleTracker_BusyCheckPreventsIdle(t *testing.T) {
	tracker := NewIdleTracker(50 * time.Millisecond)
	tracker.AddBusyCheck("always-busy", func() bool { return true })

	time.Sleep(60 * time.Millisecond)
	assert.Assert(t, !tracker.IsIdle(), "busy-check returning true should prevent idle")
}

func TestIdleTracker_BusyCheckResetsTimer(t *testing.T) {
	tracker := NewIdleTracker(50 * time.Millisecond)
	busy := true
	tracker.AddBusyCheck("togglable", func() bool { return busy })

	time.Sleep(60 * time.Millisecond)
	assert.Assert(t, !tracker.IsIdle(), "should not be idle while busy-check is true")

	// Turn off the busy-check. The idle timer should have been reset by
	// the previous IsIdle() call, so we need to wait the full timeout again.
	busy = false
	assert.Assert(t, !tracker.IsIdle(), "should not be idle immediately after busy-check clears")
	time.Sleep(60 * time.Millisecond)
	assert.Assert(t, tracker.IsIdle(), "should be idle after timeout once busy-check is false")
}

func TestIdleTracker_BusyCheckFalseAllowsIdle(t *testing.T) {
	tracker := NewIdleTracker(50 * time.Millisecond)
	tracker.AddBusyCheck("never-busy", func() bool { return false })

	time.Sleep(60 * time.Millisecond)
	assert.Assert(t, tracker.IsIdle(), "busy-check returning false should not prevent idle")
}

func TestIdleTracker_MultipleBusyChecks(t *testing.T) {
	tracker := NewIdleTracker(50 * time.Millisecond)
	checkA := true
	checkB := false
	tracker.AddBusyCheck("check-a", func() bool { return checkA })
	tracker.AddBusyCheck("check-b", func() bool { return checkB })

	time.Sleep(60 * time.Millisecond)
	// Any-true semantics: check-a is true, so not idle.
	assert.Assert(t, !tracker.IsIdle(), "any-true: should not be idle when check-a is true")

	// Turn off check-a, turn on check-b.
	checkA = false
	checkB = true
	time.Sleep(60 * time.Millisecond)
	assert.Assert(t, !tracker.IsIdle(), "any-true: should not be idle when check-b is true")

	// Both false.
	checkB = false
	time.Sleep(60 * time.Millisecond)
	assert.Assert(t, tracker.IsIdle(), "should be idle when all busy-checks are false")
}

func TestIdleTracker_AddBusyCheckConcurrent(_ *testing.T) {
	tracker := NewIdleTracker(1 * time.Second)
	done := make(chan struct{})
	go func() {
		for range 50 {
			tracker.AddBusyCheck("concurrent", func() bool { return false })
		}
		close(done)
	}()
	for range 50 {
		_ = tracker.IsIdle()
	}
	<-done
}
