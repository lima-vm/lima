// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package hostagent

import (
	"testing"
	"time"

	"gotest.tools/v3/assert"

	"github.com/lima-vm/lima/v2/pkg/guestagent/api"
)

func newTestConfig() BalloonConfig {
	return BalloonConfig{
		MaxMemoryBytes:        12 * 1024 * 1024 * 1024, // 12 GiB
		MinBytes:              3 * 1024 * 1024 * 1024,  // 3 GiB
		IdleTargetBytes:       4 * 1024 * 1024 * 1024,  // 4 GiB
		GrowStepPercent:       25,
		ShrinkStepPercent:     10,
		HighPressureThreshold: 0.88,
		LowPressureThreshold:  0.35,
		Cooldown:              30 * time.Second,
		IdleGracePeriod:       5 * time.Minute,
	}
}

func idleMetrics() *api.MemoryMetrics {
	return &api.MemoryMetrics{
		MemTotalBytes:     12 * 1024 * 1024 * 1024,
		MemAvailableBytes: 10 * 1024 * 1024 * 1024,
		PsiMemorySome_10:  0.1,
		PsiMemoryFull_10:  0.0,
		AnonRssBytes:      1 * 1024 * 1024 * 1024,
		ContainerCount:    0,
	}
}

func pressureMetrics() *api.MemoryMetrics {
	return &api.MemoryMetrics{
		MemTotalBytes:     12 * 1024 * 1024 * 1024,
		MemAvailableBytes: 1 * 1024 * 1024 * 1024,
		PsiMemorySome_10:  0.95,
		PsiMemoryFull_10:  0.80,
		AnonRssBytes:      10 * 1024 * 1024 * 1024,
		ContainerCount:    5,
	}
}

func TestBalloonController_IdleShrink(t *testing.T) {
	cfg := newTestConfig()
	ctrl := NewBalloonController(cfg)

	// After bootstrap, feed idle metrics. Current allocation should trend toward idleTarget.
	ctrl.TransitionTo(BalloonStateSteady)
	action := ctrl.Evaluate(idleMetrics(), time.Now().Add(-6*time.Minute))
	assert.Equal(t, action.Type, BalloonActionShrink)
	assert.Assert(t, action.TargetBytes <= cfg.IdleTargetBytes)
	assert.Assert(t, action.TargetBytes >= cfg.MinBytes)
}

func TestBalloonController_NoOscillation(t *testing.T) {
	cfg := newTestConfig()
	ctrl := NewBalloonController(cfg)
	ctrl.TransitionTo(BalloonStateSteady)

	// Grow due to pressure.
	action := ctrl.Evaluate(pressureMetrics(), time.Now())
	assert.Equal(t, action.Type, BalloonActionGrow)

	// Immediately after grow, idle metrics should NOT trigger shrink (cooldown).
	ctrl.RecordAction(action, time.Now())
	action2 := ctrl.Evaluate(idleMetrics(), time.Now())
	assert.Equal(t, action2.Type, BalloonActionNone)
}

func TestBalloonController_FastGrow(t *testing.T) {
	cfg := newTestConfig()
	ctrl := NewBalloonController(cfg)
	ctrl.TransitionTo(BalloonStateSteady)
	ctrl.currentBytes = cfg.IdleTargetBytes

	// High pressure should trigger immediate grow.
	action := ctrl.Evaluate(pressureMetrics(), time.Now().Add(-6*time.Minute))
	assert.Equal(t, action.Type, BalloonActionGrow)
	assert.Assert(t, action.TargetBytes > cfg.IdleTargetBytes)
}

func TestBalloonController_OOMRecovery(t *testing.T) {
	cfg := newTestConfig()
	ctrl := NewBalloonController(cfg)
	ctrl.TransitionTo(BalloonStateSteady)
	ctrl.currentBytes = 6 * 1024 * 1024 * 1024

	// OOM detected should grow to previous + 20%.
	oomMetrics := pressureMetrics()
	oomMetrics.OomDetected = true
	action := ctrl.Evaluate(oomMetrics, time.Now().Add(-6*time.Minute))
	assert.Equal(t, action.Type, BalloonActionGrow)
	expectedMin := uint64(float64(ctrl.currentBytes) * 1.20)
	assert.Assert(t, action.TargetBytes >= expectedMin)
}

func TestBalloonController_CircuitBreaker(t *testing.T) {
	cfg := newTestConfig()
	ctrl := NewBalloonController(cfg)
	ctrl.TransitionTo(BalloonStateSteady)

	// 3 OOMs in 10 minutes should lock at max.
	// Note: Evaluate() calls RecordOOM() internally when OomDetected is true.
	now := time.Now()
	for i := range 3 {
		oomMetrics := pressureMetrics()
		oomMetrics.OomDetected = true
		action := ctrl.Evaluate(oomMetrics, now.Add(-6*time.Minute))
		ctrl.RecordAction(action, now.Add(time.Duration(i)*time.Minute))
	}
	assert.Equal(t, ctrl.state, BalloonStateCircuitBreaker)
	assert.Equal(t, ctrl.currentBytes, cfg.MaxMemoryBytes)
}

func TestBalloonController_HardFloor(t *testing.T) {
	cfg := newTestConfig()
	ctrl := NewBalloonController(cfg)
	ctrl.TransitionTo(BalloonStateSteady)
	ctrl.currentBytes = 10 * 1024 * 1024 * 1024 // 10 GiB — well above idle target

	// Hard floor = max(min, anon_rss * 1.15).
	metrics := idleMetrics()
	metrics.AnonRssBytes = 4 * 1024 * 1024 * 1024 // 4 GiB anon RSS
	action := ctrl.Evaluate(metrics, time.Now().Add(-6*time.Minute))

	hardFloor := max(uint64(float64(metrics.AnonRssBytes)*1.15), cfg.MinBytes)
	assert.Equal(t, action.Type, BalloonActionShrink)
	assert.Assert(t, action.TargetBytes >= hardFloor)
}

func TestBalloonController_GracefulShutdown(t *testing.T) {
	cfg := newTestConfig()
	ctrl := NewBalloonController(cfg)
	ctrl.TransitionTo(BalloonStateSteady)
	ctrl.currentBytes = cfg.IdleTargetBytes

	// Shutdown should grow to max.
	action := ctrl.PrepareShutdown()
	assert.Equal(t, action.Type, BalloonActionGrow)
	assert.Equal(t, action.TargetBytes, cfg.MaxMemoryBytes)
}

func TestBalloonController_BootstrapTimeout(t *testing.T) {
	cfg := newTestConfig()
	ctrl := NewBalloonController(cfg)

	// In bootstrap state, should stay at max.
	assert.Equal(t, ctrl.state, BalloonStateBootstrap)
	assert.Equal(t, ctrl.currentBytes, cfg.MaxMemoryBytes)
}

func TestBalloonController_AgentFailure(t *testing.T) {
	cfg := newTestConfig()
	ctrl := NewBalloonController(cfg)
	ctrl.TransitionTo(BalloonStateSteady)
	ctrl.currentBytes = cfg.IdleTargetBytes

	// 3 consecutive poll failures should expand to max.
	for range 3 {
		ctrl.RecordPollFailure()
	}
	assert.Equal(t, ctrl.state, BalloonStateAgentFailure)
	assert.Equal(t, ctrl.currentBytes, cfg.MaxMemoryBytes)
}

// --- Edge case tests ---

func TestBalloonController_ZeroMetrics(t *testing.T) {
	// All-zero metrics should not panic or produce invalid actions.
	cfg := newTestConfig()
	ctrl := NewBalloonController(cfg)
	ctrl.TransitionTo(BalloonStateSteady)

	m := &api.MemoryMetrics{} // All zeros.
	action := ctrl.Evaluate(m, time.Now().Add(-6*time.Minute))
	// Zero PSI < lowThreshold, so it tries to shrink.
	// But anon_rss=0 → hardFloor = max(0, min) = min.
	// currentBytes(max) > idleTarget, so it shrinks.
	assert.Assert(t, action.Type == BalloonActionShrink || action.Type == BalloonActionNone)
	if action.Type == BalloonActionShrink {
		assert.Assert(t, action.TargetBytes >= cfg.MinBytes)
	}
}

func TestBalloonController_PressureExactlyAtThreshold(t *testing.T) {
	cfg := newTestConfig()
	ctrl := NewBalloonController(cfg)
	ctrl.TransitionTo(BalloonStateSteady)
	ctrl.currentBytes = cfg.IdleTargetBytes

	// PSI exactly at high threshold should trigger grow.
	m := idleMetrics()
	m.PsiMemorySome_10 = cfg.HighPressureThreshold
	action := ctrl.Evaluate(m, time.Now().Add(-6*time.Minute))
	assert.Equal(t, action.Type, BalloonActionGrow)
}

func TestBalloonController_PressureJustBelowHigh(t *testing.T) {
	cfg := newTestConfig()
	ctrl := NewBalloonController(cfg)
	ctrl.TransitionTo(BalloonStateSteady)
	ctrl.currentBytes = 8 * 1024 * 1024 * 1024

	// PSI just below high threshold: no grow, no shrink (in between thresholds).
	m := idleMetrics()
	m.PsiMemorySome_10 = cfg.HighPressureThreshold - 0.01
	action := ctrl.Evaluate(m, time.Now().Add(-6*time.Minute))
	// Between low and high → no action.
	assert.Equal(t, action.Type, BalloonActionNone)
}

func TestBalloonController_PressureExactlyAtLow(t *testing.T) {
	cfg := newTestConfig()
	ctrl := NewBalloonController(cfg)
	ctrl.TransitionTo(BalloonStateSteady)

	// PSI exactly at low threshold should NOT trigger shrink (< not <=).
	m := idleMetrics()
	m.PsiMemorySome_10 = cfg.LowPressureThreshold
	action := ctrl.Evaluate(m, time.Now().Add(-6*time.Minute))
	assert.Equal(t, action.Type, BalloonActionNone)
}

func TestBalloonController_AlreadyAtMin(t *testing.T) {
	cfg := newTestConfig()
	ctrl := NewBalloonController(cfg)
	ctrl.TransitionTo(BalloonStateSteady)
	ctrl.currentBytes = cfg.MinBytes // Already at min.

	m := idleMetrics()
	m.AnonRssBytes = 1 * 1024 * 1024 * 1024 // 1 GiB anon RSS.
	action := ctrl.Evaluate(m, time.Now().Add(-6*time.Minute))
	// currentBytes <= idleTarget → return none (already at or below idle target).
	assert.Equal(t, action.Type, BalloonActionNone)
}

func TestBalloonController_AlreadyAtMax(t *testing.T) {
	cfg := newTestConfig()
	ctrl := NewBalloonController(cfg)
	ctrl.TransitionTo(BalloonStateSteady)
	ctrl.currentBytes = cfg.MaxMemoryBytes

	// Grow when already at max should cap at max.
	m := pressureMetrics()
	action := ctrl.Evaluate(m, time.Now().Add(-6*time.Minute))
	assert.Equal(t, action.Type, BalloonActionGrow)
	assert.Equal(t, action.TargetBytes, cfg.MaxMemoryBytes)
}

func TestBalloonController_OOMAtMax(t *testing.T) {
	cfg := newTestConfig()
	ctrl := NewBalloonController(cfg)
	ctrl.TransitionTo(BalloonStateSteady)
	ctrl.currentBytes = cfg.MaxMemoryBytes

	// OOM when already at max: target is 120% of max, capped to max.
	m := pressureMetrics()
	m.OomDetected = true
	action := ctrl.Evaluate(m, time.Now().Add(-6*time.Minute))
	assert.Equal(t, action.Type, BalloonActionGrow)
	assert.Equal(t, action.TargetBytes, cfg.MaxMemoryBytes)
}

func TestBalloonController_CircuitBreakerRecovery(t *testing.T) {
	cfg := newTestConfig()
	ctrl := NewBalloonController(cfg)
	ctrl.TransitionTo(BalloonStateSteady)

	// Trigger circuit breaker via Evaluate (matches production code path).
	now := time.Now()
	for i := range 3 {
		oomMetrics := pressureMetrics()
		oomMetrics.OomDetected = true
		action := ctrl.Evaluate(oomMetrics, now.Add(-6*time.Minute))
		ctrl.RecordAction(action, now.Add(time.Duration(i)*time.Minute))
	}
	assert.Equal(t, ctrl.state, BalloonStateCircuitBreaker)

	// Before 30 min, should stay locked.
	ctrl.circuitBreakerT = now.Add(-29 * time.Minute)
	action := ctrl.Evaluate(idleMetrics(), now.Add(-6*time.Minute))
	assert.Equal(t, action.Type, BalloonActionNone)
	assert.Equal(t, ctrl.state, BalloonStateCircuitBreaker)

	// After 30 min, should recover to steady.
	ctrl.circuitBreakerT = now.Add(-31 * time.Minute)
	action = ctrl.Evaluate(idleMetrics(), now.Add(-6*time.Minute))
	assert.Equal(t, ctrl.state, BalloonStateSteady)
}

func TestBalloonController_CooldownExactlyExpired(t *testing.T) {
	cfg := newTestConfig()
	ctrl := NewBalloonController(cfg)
	ctrl.TransitionTo(BalloonStateSteady)

	// Record action exactly cooldown ago.
	ctrl.RecordAction(BalloonAction{Type: BalloonActionShrink, TargetBytes: 8 * 1024 * 1024 * 1024}, time.Now().Add(-cfg.Cooldown))
	action := ctrl.Evaluate(idleMetrics(), time.Now().Add(-6*time.Minute))
	// Cooldown check uses `<`, so exactly at cooldown should pass.
	assert.Equal(t, action.Type, BalloonActionShrink)
}

func TestBalloonController_SwapActivityBlocksShrink(t *testing.T) {
	cfg := newTestConfig()
	cfg.MaxSwapInPerSec = 64 * 1024 * 1024 // 64 MiB/s.
	ctrl := NewBalloonController(cfg)
	ctrl.TransitionTo(BalloonStateSteady)

	m := idleMetrics()
	m.SwapInBytesPerSec = 100 * 1024 * 1024 // 100 MiB/s — above threshold.
	action := ctrl.Evaluate(m, time.Now().Add(-6*time.Minute))
	assert.Equal(t, action.Type, BalloonActionNone)
}

func TestBalloonController_ContainerActivityBlocksShrink(t *testing.T) {
	cfg := newTestConfig()
	cfg.MaxContainerCPU = 10.0
	ctrl := NewBalloonController(cfg)
	ctrl.TransitionTo(BalloonStateSteady)

	m := idleMetrics()
	m.ContainerCount = 2
	m.ContainerCpuPercent = 15.0 // Above threshold.
	action := ctrl.Evaluate(m, time.Now().Add(-6*time.Minute))
	assert.Equal(t, action.Type, BalloonActionNone)
}

func TestBalloonController_PollFailureResets(t *testing.T) {
	cfg := newTestConfig()
	ctrl := NewBalloonController(cfg)
	ctrl.TransitionTo(BalloonStateSteady)

	// 2 failures, then success — should stay in steady.
	ctrl.RecordPollFailure()
	ctrl.RecordPollFailure()
	ctrl.RecordPollSuccess()
	assert.Equal(t, ctrl.state, BalloonStateSteady)

	// Need 3 more failures to trigger.
	ctrl.RecordPollFailure()
	assert.Equal(t, ctrl.state, BalloonStateSteady) // Not yet.
}

func TestBalloonController_ShrinkStepLargerThanCurrent(t *testing.T) {
	// When shrinkStep > currentBytes, target should clamp to min.
	cfg := newTestConfig()
	cfg.ShrinkStepPercent = 100 // 100% of max = 12 GiB step.
	ctrl := NewBalloonController(cfg)
	ctrl.TransitionTo(BalloonStateSteady)
	ctrl.currentBytes = 5 * 1024 * 1024 * 1024 // 5 GiB.

	m := idleMetrics()
	action := ctrl.Evaluate(m, time.Now().Add(-6*time.Minute))
	assert.Equal(t, action.Type, BalloonActionShrink)
	assert.Assert(t, action.TargetBytes >= cfg.MinBytes)
}

func TestBalloonController_IdleGracePeriodBlocksShrink(t *testing.T) {
	cfg := newTestConfig()
	cfg.IdleGracePeriod = 5 * time.Minute
	ctrl := NewBalloonController(cfg)
	ctrl.TransitionTo(BalloonStateSteady)

	// Boot time is recent — within grace period.
	action := ctrl.Evaluate(idleMetrics(), time.Now().Add(-2*time.Minute))
	assert.Equal(t, action.Type, BalloonActionNone)
}

func TestBalloonController_NilMetrics(t *testing.T) {
	cfg := newTestConfig()
	ctrl := NewBalloonController(cfg)
	ctrl.TransitionTo(BalloonStateSteady)
	// nil metrics should not panic; returns no-op.
	action := ctrl.Evaluate(nil, time.Now().Add(-10*time.Minute))
	assert.Equal(t, action.Type, BalloonActionNone)
}

func TestBalloonController_IntegerMathOOM(t *testing.T) {
	// Verify OOM growth uses integer math (currentBytes + currentBytes/5).
	cfg := newTestConfig()
	cfg.MaxMemoryBytes = 20 * 1024 * 1024 * 1024 // 20 GiB.
	ctrl := NewBalloonController(cfg)
	ctrl.TransitionTo(BalloonStateSteady)
	// Set current to 10 GiB via RecordAction.
	ctrl.RecordAction(BalloonAction{Type: BalloonActionGrow, TargetBytes: 10 * 1024 * 1024 * 1024}, time.Time{})

	m := &api.MemoryMetrics{
		OomDetected:      true,
		PsiMemorySome_10: 0.1,
	}
	action := ctrl.Evaluate(m, time.Now().Add(-10*time.Minute))
	assert.Equal(t, action.Type, BalloonActionGrow)
	// 10 GiB + 10 GiB/5 = 12 GiB.
	expected := uint64(12 * 1024 * 1024 * 1024)
	assert.Equal(t, action.TargetBytes, expected)
}

func TestBalloonController_IntegerMathGrowStep(t *testing.T) {
	// Verify grow step uses integer math (maxBytes/100 * percent).
	cfg := newTestConfig()
	cfg.MaxMemoryBytes = 12 * 1024 * 1024 * 1024 // 12 GiB.
	cfg.GrowStepPercent = 25
	ctrl := NewBalloonController(cfg)
	ctrl.TransitionTo(BalloonStateSteady)
	// Set current to 8 GiB via RecordAction.
	ctrl.RecordAction(BalloonAction{Type: BalloonActionGrow, TargetBytes: 8 * 1024 * 1024 * 1024}, time.Time{})

	m := &api.MemoryMetrics{PsiMemorySome_10: 0.95} // High pressure.
	action := ctrl.Evaluate(m, time.Now().Add(-10*time.Minute))
	assert.Equal(t, action.Type, BalloonActionGrow)
	// step = 12 GiB / 100 * 25 (integer division); target = 8 GiB + step.
	step := uint64(12*1024*1024*1024) / 100 * 25
	expected := uint64(8*1024*1024*1024) + step
	assert.Equal(t, action.TargetBytes, expected)
}
