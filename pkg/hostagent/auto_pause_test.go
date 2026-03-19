// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package hostagent

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"gotest.tools/v3/assert"

	"github.com/lima-vm/lima/v2/pkg/guestagent/api"
)

// mockPausable implements driver.Pausable for testing.
type mockPausable struct {
	mu        sync.Mutex
	paused    bool
	pauseErr  error
	resumeErr error
}

func (m *mockPausable) Pause(_ context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.pauseErr != nil {
		return m.pauseErr
	}
	m.paused = true
	return nil
}

func (m *mockPausable) Resume(_ context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.resumeErr != nil {
		return m.resumeErr
	}
	m.paused = false
	return nil
}

func (m *mockPausable) IsPaused() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.paused
}

func TestAutoPauseManager_PausesOnIdle(t *testing.T) {
	mock := &mockPausable{}
	mgr := NewAutoPauseManager(mock, 50*time.Millisecond, 5*time.Second, DefaultIdleSignalConfig())
	mgr.tickInterval = 20 * time.Millisecond // Speed up for test.
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	go mgr.Run(ctx)

	// Wait for idle timeout + tick to trigger.
	time.Sleep(200 * time.Millisecond)

	assert.Assert(t, mock.IsPaused(), "VM should be paused after idle timeout")
}

func TestAutoPauseManager_TouchPreventsIdle(t *testing.T) {
	mock := &mockPausable{}
	mgr := NewAutoPauseManager(mock, 80*time.Millisecond, 5*time.Second, DefaultIdleSignalConfig())
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	go mgr.Run(ctx)

	// Keep touching to prevent idle.
	for range 5 {
		time.Sleep(40 * time.Millisecond)
		mgr.Touch()
	}

	assert.Assert(t, !mock.IsPaused(), "VM should not be paused when activity is ongoing")
}

func TestAutoPauseManager_ResumeOnTouch(t *testing.T) {
	mock := &mockPausable{}
	mgr := NewAutoPauseManager(mock, 50*time.Millisecond, 5*time.Second, DefaultIdleSignalConfig())
	mgr.tickInterval = 20 * time.Millisecond // Speed up for test.
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	go mgr.Run(ctx)

	// Wait for pause.
	time.Sleep(200 * time.Millisecond)
	assert.Assert(t, mock.IsPaused(), "VM should be paused")

	// Touch should resume.
	mgr.Touch()
	time.Sleep(50 * time.Millisecond) // Give resume goroutine time.
	assert.Assert(t, !mock.IsPaused(), "VM should be resumed after Touch")
}

func TestAutoPauseManager_ContextCancel(t *testing.T) {
	mock := &mockPausable{}
	mgr := NewAutoPauseManager(mock, 1*time.Hour, 5*time.Second, DefaultIdleSignalConfig())
	ctx, cancel := context.WithCancel(t.Context())

	done := make(chan struct{})
	go func() {
		mgr.Run(ctx)
		close(done)
	}()

	cancel()
	select {
	case <-done:
		// Run exited cleanly.
	case <-time.After(2 * time.Second):
		assert.Assert(t, false, "Run did not exit after context cancel")
	}
}

// --- Edge case tests ---

func TestAutoPauseManager_PauseError(t *testing.T) {
	mock := &mockPausable{pauseErr: errors.New("pause failed")}
	mgr := NewAutoPauseManager(mock, 50*time.Millisecond, 5*time.Second, DefaultIdleSignalConfig())
	mgr.tickInterval = 20 * time.Millisecond
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	go mgr.Run(ctx)
	time.Sleep(200 * time.Millisecond)

	// Pause failed, so VM should NOT be paused.
	assert.Assert(t, !mock.IsPaused(), "VM should not be paused when Pause() returns error")
}

func TestAutoPauseManager_RapidTouchWhilePaused(t *testing.T) {
	mock := &mockPausable{}
	mgr := NewAutoPauseManager(mock, 50*time.Millisecond, 5*time.Second, DefaultIdleSignalConfig())
	mgr.tickInterval = 20 * time.Millisecond
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	go mgr.Run(ctx)

	// Wait for pause.
	time.Sleep(200 * time.Millisecond)
	assert.Assert(t, mock.IsPaused())

	// Rapid-fire Touch() calls should not panic.
	for range 20 {
		mgr.Touch()
	}
	time.Sleep(50 * time.Millisecond)
	assert.Assert(t, !mock.IsPaused(), "VM should be resumed after rapid Touch calls")
}

func TestAutoPauseManager_ResumeOnShutdownWhilePaused(t *testing.T) {
	mock := &mockPausable{}
	mgr := NewAutoPauseManager(mock, 50*time.Millisecond, 5*time.Second, DefaultIdleSignalConfig())
	mgr.tickInterval = 20 * time.Millisecond
	ctx, cancel := context.WithCancel(t.Context())

	done := make(chan struct{})
	go func() {
		mgr.Run(ctx)
		close(done)
	}()

	// Wait for pause.
	time.Sleep(200 * time.Millisecond)
	assert.Assert(t, mock.IsPaused())

	// Cancel context — Run should resume VM before exiting.
	cancel()
	<-done
	assert.Assert(t, !mock.IsPaused(), "VM should be resumed on shutdown")
}

func TestAutoPauseManager_TouchAlwaysSendsSignal(_ *testing.T) {
	// Touch should always try to signal resumeCh, even if not paused.
	// This ensures no race between IsPaused check and channel send.
	mock := &mockPausable{}
	mgr := NewAutoPauseManager(mock, 50*time.Millisecond, 5*time.Second, DefaultIdleSignalConfig())
	// Touch when not paused should not block or panic.
	for range 100 {
		mgr.Touch()
	}
}

// --- WaitForRunning tests ---

func TestWaitForRunning_ReturnsImmediatelyWhenNotPaused(t *testing.T) {
	mock := &mockPausable{} // not paused
	mgr := NewAutoPauseManager(mock, 1*time.Hour, 5*time.Second, DefaultIdleSignalConfig())

	err := mgr.WaitForRunning(t.Context())
	assert.NilError(t, err)
}

func TestWaitForRunning_BlocksUntilResumed(t *testing.T) {
	mock := &mockPausable{paused: true}
	mgr := NewAutoPauseManager(mock, 1*time.Hour, 5*time.Second, DefaultIdleSignalConfig())

	// Resume after 100ms in a goroutine.
	go func() {
		time.Sleep(100 * time.Millisecond)
		mock.mu.Lock()
		mock.paused = false
		mock.mu.Unlock()
	}()

	err := mgr.WaitForRunning(t.Context())
	assert.NilError(t, err)
}

func TestWaitForRunning_RespectsContextCancellation(t *testing.T) {
	mock := &mockPausable{paused: true}
	mgr := NewAutoPauseManager(mock, 1*time.Hour, 5*time.Second, DefaultIdleSignalConfig())

	ctx, cancel := context.WithCancel(t.Context())
	// Cancel after 50ms.
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	err := mgr.WaitForRunning(ctx)
	assert.ErrorIs(t, err, context.Canceled)
}

// --- OnResume callback tests ---

func TestOnResumeCallback_FiresAfterResume(t *testing.T) {
	mock := &mockPausable{}
	mgr := NewAutoPauseManager(mock, 50*time.Millisecond, 5*time.Second, DefaultIdleSignalConfig())
	mgr.tickInterval = 20 * time.Millisecond

	var callbackCalled atomic.Bool
	mgr.onResume = func() { callbackCalled.Store(true) }

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	go mgr.Run(ctx)

	// Wait for pause.
	time.Sleep(200 * time.Millisecond)
	assert.Assert(t, mock.IsPaused())

	// Touch triggers resume → onResume should fire.
	mgr.Touch()
	time.Sleep(100 * time.Millisecond)

	assert.Assert(t, callbackCalled.Load(), "OnResume callback should have been called")
}

func TestOnResumeCallback_NotCalledOnPause(t *testing.T) {
	mock := &mockPausable{}
	mgr := NewAutoPauseManager(mock, 50*time.Millisecond, 5*time.Second, DefaultIdleSignalConfig())
	mgr.tickInterval = 20 * time.Millisecond

	var callbackCalled atomic.Bool
	mgr.onResume = func() { callbackCalled.Store(true) }

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	go mgr.Run(ctx)

	// Wait for pause only.
	time.Sleep(200 * time.Millisecond)
	assert.Assert(t, mock.IsPaused())
	assert.Assert(t, !callbackCalled.Load(), "OnResume should not be called on pause")
}

func TestOnResumeCallback_NilCallbackSafe(t *testing.T) {
	mock := &mockPausable{}
	mgr := NewAutoPauseManager(mock, 50*time.Millisecond, 5*time.Second, DefaultIdleSignalConfig())
	mgr.tickInterval = 20 * time.Millisecond
	// onResume is nil — should not panic.

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	go mgr.Run(ctx)

	time.Sleep(200 * time.Millisecond) // pause
	mgr.Touch()                        // resume
	time.Sleep(100 * time.Millisecond) // no panic
}

func TestOnResumeCallback_NotCalledOnFailedResume(t *testing.T) {
	mock := &mockPausable{resumeErr: errors.New("resume failed")}
	mgr := NewAutoPauseManager(mock, 50*time.Millisecond, 5*time.Second, DefaultIdleSignalConfig())
	mgr.tickInterval = 20 * time.Millisecond

	var callbackCalled atomic.Bool
	mgr.onResume = func() { callbackCalled.Store(true) }

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	go mgr.Run(ctx)

	// Wait for pause (pauseErr is nil, so Pause succeeds).
	time.Sleep(200 * time.Millisecond)
	assert.Assert(t, mock.IsPaused())

	// Touch to attempt resume — but Resume() returns error.
	mgr.Touch()
	time.Sleep(100 * time.Millisecond)

	assert.Assert(t, !callbackCalled.Load(), "OnResume should not be called when Resume fails")
}

func TestAutoPauseManager_ShutdownUsesBackgroundContext(t *testing.T) {
	// Ensure shutdown resume uses a fresh context (not the cancelled one).
	mock := &mockPausable{}
	mgr := NewAutoPauseManager(mock, 50*time.Millisecond, 5*time.Second, DefaultIdleSignalConfig())
	mgr.tickInterval = 20 * time.Millisecond
	ctx, cancel := context.WithCancel(t.Context())

	done := make(chan struct{})
	go func() {
		mgr.Run(ctx)
		close(done)
	}()

	// Wait for pause.
	time.Sleep(200 * time.Millisecond)
	assert.Assert(t, mock.IsPaused(), "VM should be paused after idle")

	// Cancel — Run should resume even though ctx is done.
	cancel()
	<-done
	assert.Assert(t, !mock.IsPaused(), "VM should be resumed on shutdown")
}

// --- Wake detection tests ---

func TestWakeDetection_DetectsLargeTimeGap(t *testing.T) {
	mock := &mockPausable{}
	mgr := NewAutoPauseManager(mock, 1*time.Hour, 5*time.Second, DefaultIdleSignalConfig())

	var wakeCalled atomic.Bool
	mgr.onWake = func() { wakeCalled.Store(true) }

	// Initialize lastTick to a time 10s ago — simulates system sleep.
	mgr.lastTick = time.Now().Add(-10 * time.Second)

	// checkWake should detect the large gap.
	assert.Assert(t, mgr.checkWake(time.Now()), "should detect wake with 10s gap")
	assert.Assert(t, wakeCalled.Load(), "onWake should have been called")
}

func TestWakeDetection_IgnoresNormalJitter(t *testing.T) {
	mock := &mockPausable{}
	mgr := NewAutoPauseManager(mock, 1*time.Hour, 5*time.Second, DefaultIdleSignalConfig())

	var wakeCalled atomic.Bool
	mgr.onWake = func() { wakeCalled.Store(true) }

	// Set lastTick to 100ms ago — normal jitter.
	mgr.lastTick = time.Now().Add(-100 * time.Millisecond)

	assert.Assert(t, !mgr.checkWake(time.Now()), "should not detect wake with 100ms gap")
	assert.Assert(t, !wakeCalled.Load(), "onWake should not have been called")
}

func TestWakeDetection_ResetsIdleTimer(t *testing.T) {
	mock := &mockPausable{}
	mgr := NewAutoPauseManager(mock, 50*time.Millisecond, 5*time.Second, DefaultIdleSignalConfig())

	// Let the idle tracker go idle.
	time.Sleep(100 * time.Millisecond)
	assert.Assert(t, mgr.idleTracker.IsIdle(), "should be idle before wake")

	// Simulate wake — should reset idle timer.
	mgr.lastTick = time.Now().Add(-10 * time.Second)
	mgr.checkWake(time.Now())

	assert.Assert(t, !mgr.idleTracker.IsIdle(), "idle timer should be reset after wake detection")
}

func TestWakeDetection_NilCallbackSafe(_ *testing.T) {
	mock := &mockPausable{}
	mgr := NewAutoPauseManager(mock, 1*time.Hour, 5*time.Second, DefaultIdleSignalConfig())
	// onWake is nil — should not panic.

	mgr.lastTick = time.Now().Add(-10 * time.Second)
	mgr.checkWake(time.Now()) // no panic
}

func TestWakeDetection_NotCalledWhenVMPaused(t *testing.T) {
	mock := &mockPausable{paused: true}
	mgr := NewAutoPauseManager(mock, 1*time.Hour, 5*time.Second, DefaultIdleSignalConfig())

	var wakeCalled atomic.Bool
	mgr.onWake = func() { wakeCalled.Store(true) }

	mgr.lastTick = time.Now().Add(-10 * time.Second)
	mgr.checkWake(time.Now())

	assert.Assert(t, !wakeCalled.Load(), "onWake should not be called when VM is paused")
}

func TestWakeDetection_NoFalsePositiveOnFirstTick(t *testing.T) {
	mock := &mockPausable{}
	mgr := NewAutoPauseManager(mock, 1*time.Hour, 5*time.Second, DefaultIdleSignalConfig())
	mgr.tickInterval = 10 * time.Millisecond

	var wakeCalled atomic.Bool
	mgr.onWake = func() { wakeCalled.Store(true) }

	ctx, cancel := context.WithCancel(t.Context())
	go mgr.Run(ctx)

	// Wait for a few ticks.
	time.Sleep(100 * time.Millisecond)
	cancel()

	assert.Assert(t, !wakeCalled.Load(), "onWake should not fire on first tick")
}

// --- Guest Metrics Busy-Check Tests ---

func TestAutoPause_ContainerActivityPreventsIdle(t *testing.T) {
	mock := &mockPausable{}
	mgr := NewAutoPauseManager(mock, 50*time.Millisecond, 30*time.Second, DefaultIdleSignalConfig())

	// Register the split container busy-checks.
	mgr.AddBusyCheck("container-cpu", mgr.hasContainerCPUActivity)
	mgr.AddBusyCheck("container-io", mgr.hasContainerIOActivity)

	// Update with active containers.
	mgr.UpdateGuestMetrics(&api.MemoryMetrics{
		ContainerCount:      2,
		ContainerCpuPercent: 10.0,
	})

	time.Sleep(60 * time.Millisecond)
	assert.Assert(t, !mgr.idleTracker.IsIdle(), "should not be idle with active containers")
}

func TestAutoPause_IdleContainersAllowPause(t *testing.T) {
	mock := &mockPausable{}
	mgr := NewAutoPauseManager(mock, 50*time.Millisecond, 30*time.Second, DefaultIdleSignalConfig())
	mgr.AddBusyCheck("container-cpu", mgr.hasContainerCPUActivity)
	mgr.AddBusyCheck("container-io", mgr.hasContainerIOActivity)

	// Container with zero CPU and no IO change.
	mgr.UpdateGuestMetrics(&api.MemoryMetrics{
		ContainerCount:         1,
		ContainerCpuPercent:    0.0,
		ContainerIoBytesPerSec: 0.0,
	})
	// Call hasContainerIOActivity once to set prevIOBytes baseline.
	mgr.hasContainerIOActivity()

	// Update with same IO.
	mgr.UpdateGuestMetrics(&api.MemoryMetrics{
		ContainerCount:         1,
		ContainerCpuPercent:    0.0,
		ContainerIoBytesPerSec: 0.0,
	})

	time.Sleep(60 * time.Millisecond)
	assert.Assert(t, mgr.idleTracker.IsIdle(), "idle containers should allow pause")
}

func TestAutoPause_NoContainersAllowPause(t *testing.T) {
	mock := &mockPausable{}
	mgr := NewAutoPauseManager(mock, 50*time.Millisecond, 30*time.Second, DefaultIdleSignalConfig())
	mgr.AddBusyCheck("container-cpu", mgr.hasContainerCPUActivity)
	mgr.AddBusyCheck("container-io", mgr.hasContainerIOActivity)

	mgr.UpdateGuestMetrics(&api.MemoryMetrics{
		ContainerCount: 0,
	})

	time.Sleep(60 * time.Millisecond)
	assert.Assert(t, mgr.idleTracker.IsIdle(), "no containers should allow pause")
}

func TestAutoPause_StaleMetricsAllowPause(t *testing.T) {
	mock := &mockPausable{}
	mgr := NewAutoPauseManager(mock, 50*time.Millisecond, 30*time.Second, DefaultIdleSignalConfig())
	mgr.AddBusyCheck("container-cpu", mgr.hasContainerCPUActivity)
	mgr.AddBusyCheck("container-io", mgr.hasContainerIOActivity)

	// Update with active containers.
	mgr.UpdateGuestMetrics(&api.MemoryMetrics{
		ContainerCount:      2,
		ContainerCpuPercent: 50.0,
	})

	// Simulate staleness by setting metricsTime to 31s ago.
	mgr.metricsMu.Lock()
	mgr.metricsTime = time.Now().Add(-31 * time.Second)
	mgr.metricsMu.Unlock()

	time.Sleep(60 * time.Millisecond)
	assert.Assert(t, mgr.idleTracker.IsIdle(), "stale metrics should allow pause")
}

func TestAutoPause_IOChangePreventsIdle(t *testing.T) {
	mock := &mockPausable{}
	mgr := NewAutoPauseManager(mock, 50*time.Millisecond, 30*time.Second, DefaultIdleSignalConfig())
	mgr.AddBusyCheck("container-io", mgr.hasContainerIOActivity)

	// First update: IO = 1000.
	mgr.UpdateGuestMetrics(&api.MemoryMetrics{
		ContainerCount:         1,
		ContainerCpuPercent:    0.0,
		ContainerIoBytesPerSec: 1000.0,
	})
	assert.Assert(t, mgr.hasContainerIOActivity(), "IO change from 0→1000 should be detected")

	// Second update: same IO = 1000 → no change.
	mgr.UpdateGuestMetrics(&api.MemoryMetrics{
		ContainerCount:         1,
		ContainerCpuPercent:    0.0,
		ContainerIoBytesPerSec: 1000.0,
	})
	assert.Assert(t, !mgr.hasContainerIOActivity(), "same IO should not be detected as change")
}

func TestAutoPause_CPUBelowThresholdAllowsPause(t *testing.T) {
	mock := &mockPausable{}
	mgr := NewAutoPauseManager(mock, 50*time.Millisecond, 30*time.Second, DefaultIdleSignalConfig())
	mgr.AddBusyCheck("container-cpu", mgr.hasContainerCPUActivity)
	mgr.AddBusyCheck("container-io", mgr.hasContainerIOActivity)

	mgr.UpdateGuestMetrics(&api.MemoryMetrics{
		ContainerCount:         1,
		ContainerCpuPercent:    0.1, // below 0.5% threshold
		ContainerIoBytesPerSec: 0.0,
	})
	// Set prevIOBytes baseline.
	mgr.hasContainerIOActivity()

	mgr.UpdateGuestMetrics(&api.MemoryMetrics{
		ContainerCount:         1,
		ContainerCpuPercent:    0.1,
		ContainerIoBytesPerSec: 0.0,
	})

	time.Sleep(60 * time.Millisecond)
	assert.Assert(t, mgr.idleTracker.IsIdle(), "CPU below threshold should allow pause")
}

// --- Phase 7: IdleSignalConfig Tests ---

func TestDefaultIdleSignalConfig(t *testing.T) {
	cfg := DefaultIdleSignalConfig()
	assert.Assert(t, cfg.ActiveConnections, "ActiveConnections should default to true")
	assert.Assert(t, cfg.ContainerCPU, "ContainerCPU should default to true")
	assert.Equal(t, cfg.ContainerCPUThreshold, 0.5)
	assert.Assert(t, cfg.ContainerIO, "ContainerIO should default to true")
}

func TestAutoPause_AllSignalsDisabled_OnlyTouchPreventsIdle(t *testing.T) {
	mock := &mockPausable{}
	cfg := IdleSignalConfig{
		ActiveConnections:     false,
		ContainerCPU:          false,
		ContainerCPUThreshold: 0.5,
		ContainerIO:           false,
	}
	mgr := NewAutoPauseManager(mock, 50*time.Millisecond, 30*time.Second, cfg)
	// Do NOT register any busy-checks (simulates hostagent skipping all registrations).

	// Metrics exist but no check reads them.
	mgr.UpdateGuestMetrics(&api.MemoryMetrics{
		ContainerCount:      2,
		ContainerCpuPercent: 50.0,
	})

	time.Sleep(60 * time.Millisecond)
	assert.Assert(t, mgr.idleTracker.IsIdle(), "should be idle with no busy-checks registered")

	// Touch still works.
	mgr.Touch()
	assert.Assert(t, !mgr.idleTracker.IsIdle(), "Touch should still reset idle timer")
}

func TestAutoPause_ContainerCPUDisabled_IOStillPrevents(t *testing.T) {
	mock := &mockPausable{}
	cfg := DefaultIdleSignalConfig()
	cfg.ContainerCPU = false
	mgr := NewAutoPauseManager(mock, 50*time.Millisecond, 30*time.Second, cfg)
	// Register only IO busy-check (simulates hostagent conditional wiring).
	mgr.AddBusyCheck("container-io", mgr.hasContainerIOActivity)

	mgr.UpdateGuestMetrics(&api.MemoryMetrics{
		ContainerCount:         1,
		ContainerCpuPercent:    50.0,
		ContainerIoBytesPerSec: 1000.0,
	})

	assert.Assert(t, !mgr.idleTracker.IsIdle(), "IO busy-check should prevent idle")
}

func TestAutoPause_ContainerIODisabled_CPUStillPrevents(t *testing.T) {
	mock := &mockPausable{}
	cfg := DefaultIdleSignalConfig()
	cfg.ContainerIO = false
	mgr := NewAutoPauseManager(mock, 50*time.Millisecond, 30*time.Second, cfg)
	// Register only CPU busy-check (simulates hostagent conditional wiring).
	mgr.AddBusyCheck("container-cpu", mgr.hasContainerCPUActivity)

	mgr.UpdateGuestMetrics(&api.MemoryMetrics{
		ContainerCount:         1,
		ContainerCpuPercent:    50.0,
		ContainerIoBytesPerSec: 0.0,
	})

	assert.Assert(t, !mgr.idleTracker.IsIdle(), "CPU busy-check should prevent idle")
}

func TestAutoPause_ActiveConnectionsDisabled(t *testing.T) {
	mock := &mockPausable{}
	cfg := DefaultIdleSignalConfig()
	cfg.ActiveConnections = false
	mgr := NewAutoPauseManager(mock, 50*time.Millisecond, 30*time.Second, cfg)
	// Do NOT register active-connections busy-check (simulates hostagent skipping it).

	time.Sleep(60 * time.Millisecond)
	assert.Assert(t, mgr.idleTracker.IsIdle(), "should be idle with no connection busy-check")
}

func TestAutoPause_CustomCPUThreshold(t *testing.T) {
	mock := &mockPausable{}
	cfg := DefaultIdleSignalConfig()
	cfg.ContainerCPUThreshold = 5.0
	mgr := NewAutoPauseManager(mock, 1*time.Hour, 30*time.Second, cfg)

	// CPU at 2.0% — below custom threshold of 5.0%.
	mgr.UpdateGuestMetrics(&api.MemoryMetrics{
		ContainerCount:      1,
		ContainerCpuPercent: 2.0,
	})
	assert.Assert(t, !mgr.hasContainerCPUActivity(), "2.0%% CPU should be below 5.0%% threshold")

	// CPU at 6.0% — above custom threshold.
	mgr.UpdateGuestMetrics(&api.MemoryMetrics{
		ContainerCount:      1,
		ContainerCpuPercent: 6.0,
	})
	assert.Assert(t, mgr.hasContainerCPUActivity(), "6.0%% CPU should be above 5.0%% threshold")
}

func TestAutoPause_DefaultConfig_SameAsPhase6(t *testing.T) {
	mock := &mockPausable{}
	cfg := DefaultIdleSignalConfig()
	mgr := NewAutoPauseManager(mock, 50*time.Millisecond, 30*time.Second, cfg)

	// Verify all signals are enabled by default.
	assert.Assert(t, cfg.ActiveConnections)
	assert.Assert(t, cfg.ContainerCPU)
	assert.Assert(t, cfg.ContainerIO)
	assert.Equal(t, cfg.ContainerCPUThreshold, 0.5)

	// Register all busy-checks (same as Phase 6 hostagent wiring).
	mgr.AddBusyCheck("container-cpu", mgr.hasContainerCPUActivity)
	mgr.AddBusyCheck("container-io", mgr.hasContainerIOActivity)

	// With active containers, should not be idle.
	mgr.UpdateGuestMetrics(&api.MemoryMetrics{
		ContainerCount:      1,
		ContainerCpuPercent: 10.0,
	})
	assert.Assert(t, !mgr.idleTracker.IsIdle(), "default config should match Phase 6 behavior")
}

func TestAutoPause_HasContainerCPUActivity(t *testing.T) {
	mock := &mockPausable{}
	mgr := NewAutoPauseManager(mock, 1*time.Hour, 30*time.Second, DefaultIdleSignalConfig())

	// No metrics → not busy.
	assert.Assert(t, !mgr.hasContainerCPUActivity())

	// Active container above threshold.
	mgr.UpdateGuestMetrics(&api.MemoryMetrics{
		ContainerCount:      1,
		ContainerCpuPercent: 10.0,
	})
	assert.Assert(t, mgr.hasContainerCPUActivity(), "CPU above threshold should be active")

	// Below threshold.
	mgr.UpdateGuestMetrics(&api.MemoryMetrics{
		ContainerCount:      1,
		ContainerCpuPercent: 0.1,
	})
	assert.Assert(t, !mgr.hasContainerCPUActivity(), "CPU below threshold should not be active")

	// No containers.
	mgr.UpdateGuestMetrics(&api.MemoryMetrics{
		ContainerCount: 0,
	})
	assert.Assert(t, !mgr.hasContainerCPUActivity(), "no containers should not be active")
}

func TestAutoPause_HasContainerIOActivity(t *testing.T) {
	mock := &mockPausable{}
	mgr := NewAutoPauseManager(mock, 1*time.Hour, 30*time.Second, DefaultIdleSignalConfig())

	// No metrics → not busy.
	assert.Assert(t, !mgr.hasContainerIOActivity())

	// IO change from 0 → 1000.
	mgr.UpdateGuestMetrics(&api.MemoryMetrics{
		ContainerCount:         1,
		ContainerIoBytesPerSec: 1000.0,
	})
	assert.Assert(t, mgr.hasContainerIOActivity(), "IO change should be detected")

	// Same IO — no change.
	mgr.UpdateGuestMetrics(&api.MemoryMetrics{
		ContainerCount:         1,
		ContainerIoBytesPerSec: 1000.0,
	})
	assert.Assert(t, !mgr.hasContainerIOActivity(), "same IO should not be detected")

	// IO change 1000 → 2000.
	mgr.UpdateGuestMetrics(&api.MemoryMetrics{
		ContainerCount:         1,
		ContainerIoBytesPerSec: 2000.0,
	})
	assert.Assert(t, mgr.hasContainerIOActivity(), "IO increase should be detected")

	// No containers.
	mgr.UpdateGuestMetrics(&api.MemoryMetrics{
		ContainerCount: 0,
	})
	assert.Assert(t, !mgr.hasContainerIOActivity(), "no containers should not be active")
}

func TestAutoPause_ForcePause_WhenRunning(t *testing.T) {
	mock := &mockPausable{}
	mgr := NewAutoPauseManager(mock, 1*time.Hour, 30*time.Second, DefaultIdleSignalConfig())

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	go mgr.Run(ctx)

	// ForcePause should pause a running VM.
	err := mgr.ForcePause(ctx)
	assert.NilError(t, err)
	assert.Assert(t, mock.IsPaused(), "VM should be paused after ForcePause")
}

func TestAutoPause_ForcePause_WhenAlreadyPaused(t *testing.T) {
	mock := &mockPausable{}
	mgr := NewAutoPauseManager(mock, 1*time.Hour, 30*time.Second, DefaultIdleSignalConfig())

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	go mgr.Run(ctx)

	// First pause.
	err := mgr.ForcePause(ctx)
	assert.NilError(t, err)
	assert.Assert(t, mock.IsPaused())

	// Second pause is a no-op.
	err = mgr.ForcePause(ctx)
	assert.NilError(t, err)
	assert.Assert(t, mock.IsPaused(), "VM should still be paused")
}

func TestAutoPause_ForcePause_ThenResume(t *testing.T) {
	mock := &mockPausable{}
	mgr := NewAutoPauseManager(mock, 1*time.Hour, 30*time.Second, DefaultIdleSignalConfig())

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	go mgr.Run(ctx)

	// ForcePause.
	err := mgr.ForcePause(ctx)
	assert.NilError(t, err)
	assert.Assert(t, mock.IsPaused())

	// Touch triggers resume via the existing path.
	mgr.Touch()
	time.Sleep(200 * time.Millisecond) // allow Run() to process resumeCh
	assert.Assert(t, !mock.IsPaused(), "VM should be running after Touch")
}

func TestAutoPause_ForcePause_Cancellation(t *testing.T) {
	mock := &mockPausable{}
	mgr := NewAutoPauseManager(mock, 1*time.Hour, 30*time.Second, DefaultIdleSignalConfig())
	// Do NOT start Run() — forcePauseCh will never be consumed.

	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()

	err := mgr.ForcePause(ctx)
	assert.Assert(t, err != nil, "ForcePause should fail when context cancelled")
	assert.Assert(t, !mock.IsPaused(), "VM should not be paused")
}

func TestAutoPause_ForcePause_AfterRunExits(t *testing.T) {
	mock := &mockPausable{}
	mgr := NewAutoPauseManager(mock, 1*time.Hour, 30*time.Second, DefaultIdleSignalConfig())

	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})
	go func() {
		mgr.Run(ctx)
		close(done)
	}()
	// Let Run() start, then cancel to make it exit.
	time.Sleep(50 * time.Millisecond)
	cancel()
	<-done

	// ForcePause after Run() has exited should return an error, not hang.
	err := mgr.ForcePause(t.Context())
	assert.ErrorContains(t, err, "not running")
}

func TestAutoPause_ForcePause_PauseError(t *testing.T) {
	mock := &mockPausable{pauseErr: errors.New("hypervisor busy")}
	mgr := NewAutoPauseManager(mock, 1*time.Hour, 30*time.Second, DefaultIdleSignalConfig())
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	go mgr.Run(ctx)
	time.Sleep(50 * time.Millisecond)

	err := mgr.ForcePause(t.Context())
	assert.ErrorContains(t, err, "hypervisor busy")
	assert.Assert(t, !mock.IsPaused(), "VM should not be paused when Pause() errors")
}
