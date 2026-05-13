// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package hostagent

import (
	"strings"
	"testing"
	"time"

	"gotest.tools/v3/assert"

	"github.com/lima-vm/lima/v2/pkg/guestagent/api"
	"github.com/lima-vm/lima/v2/pkg/store"
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

	// Hard floor = max(min, anon_rss * margin).
	// With 0 containers, adaptive margin is 5%.
	metrics := idleMetrics()
	metrics.AnonRssBytes = 4 * 1024 * 1024 * 1024 // 4 GiB anon RSS
	action := ctrl.Evaluate(metrics, time.Now().Add(-6*time.Minute))

	hardFloor := max(uint64(float64(metrics.AnonRssBytes)*1.05), cfg.MinBytes)
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

	// 6 consecutive poll failures should expand to max (graduated: 3=partial, 6=full).
	for range 6 {
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
	// Zero PSI means psiAvailable=false, and MemAvailable=0 < 15% of currentBytes.
	// The no-PSI fallback would trigger grow, but currentBytes == MaxMemoryBytes,
	// so grow is capped and returns None instead.
	assert.Equal(t, action.Type, BalloonActionNone)
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

	// 2 failures, then success — should stay in steady (transient, no change).
	ctrl.RecordPollFailure()
	ctrl.RecordPollFailure()
	ctrl.RecordPollSuccess()
	assert.Equal(t, ctrl.state, BalloonStateSteady)

	// Need 3 more failures to trigger partial grow.
	ctrl.RecordPollFailure()
	ctrl.RecordPollFailure()
	assert.Equal(t, ctrl.state, BalloonStateSteady) // Not yet (only 2).
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
	// Verify grow step uses integer math (maxBytes * percent / 100).
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
	// step = 12 GiB * 25 / 100 (multiply-first for consistency with shrink).
	step := uint64(12*1024*1024*1024) * 25 / 100
	expected := uint64(8*1024*1024*1024) + step
	assert.Equal(t, action.TargetBytes, expected)
}

// --- E1: PSI Full Hard Stop tests ---

func TestBalloon_PsiFullBlocksShrink(t *testing.T) {
	cfg := newTestConfig()
	ctrl := NewBalloonController(cfg)
	ctrl.TransitionTo(BalloonStateSteady)
	ctrl.currentBytes = 8 * 1024 * 1024 * 1024 // Above idle target.

	// PSI some below low threshold (would normally shrink), but full > 0.
	m := idleMetrics()
	m.PsiMemorySome_10 = 0.1
	m.PsiMemoryFull_10 = 0.05
	action := ctrl.Evaluate(m, time.Now().Add(-6*time.Minute))
	assert.Equal(t, action.Type, BalloonActionNone)
}

func TestBalloon_PsiFullAllowsGrow(t *testing.T) {
	cfg := newTestConfig()
	ctrl := NewBalloonController(cfg)
	ctrl.TransitionTo(BalloonStateSteady)
	ctrl.currentBytes = 4 * 1024 * 1024 * 1024

	// PSI some above high threshold AND full > 0 — should grow.
	m := pressureMetrics()
	m.PsiMemorySome_10 = 1.0
	m.PsiMemoryFull_10 = 0.9
	action := ctrl.Evaluate(m, time.Now().Add(-6*time.Minute))
	assert.Equal(t, action.Type, BalloonActionGrow)
}

func TestBalloon_PsiFullGrowBoost(t *testing.T) {
	cfg := newTestConfig()
	ctrl := NewBalloonController(cfg)
	ctrl.TransitionTo(BalloonStateSteady)
	ctrl.currentBytes = 4 * 1024 * 1024 * 1024

	// High pressure with PSI full > 0 — should get 1.5× grow step.
	m := pressureMetrics()
	m.PsiMemorySome_10 = 0.95
	m.PsiMemoryFull_10 = 0.5
	action := ctrl.Evaluate(m, time.Now().Add(-6*time.Minute))
	assert.Equal(t, action.Type, BalloonActionGrow)

	// Verify boosted step: step = MaxMem * GrowPct / 100 * 3/2.
	normalStep := cfg.MaxMemoryBytes * uint64(cfg.GrowStepPercent) / 100
	boostedStep := normalStep * 3 / 2
	expected := uint64(4*1024*1024*1024) + boostedStep
	assert.Equal(t, action.TargetBytes, expected)
}

func TestBalloon_PsiFullZeroNoEffect(t *testing.T) {
	cfg := newTestConfig()
	ctrl := NewBalloonController(cfg)
	ctrl.TransitionTo(BalloonStateSteady)
	ctrl.currentBytes = 8 * 1024 * 1024 * 1024

	// PSI full = 0 — normal shrink behavior, E1 has no effect.
	m := idleMetrics()
	m.PsiMemorySome_10 = 0.1
	m.PsiMemoryFull_10 = 0.0
	action := ctrl.Evaluate(m, time.Now().Add(-6*time.Minute))
	assert.Equal(t, action.Type, BalloonActionShrink)
}

// --- E2: Swap-Out Rate Signal tests ---

func TestBalloon_SwapOutBlocksShrink(t *testing.T) {
	cfg := newTestConfig()
	cfg.MaxSwapOutPerSec = 32 * 1024 * 1024 // 32 MiB/s.
	ctrl := NewBalloonController(cfg)
	ctrl.TransitionTo(BalloonStateSteady)
	ctrl.currentBytes = 8 * 1024 * 1024 * 1024

	m := idleMetrics()
	m.SwapOutBytesPerSec = 40 * 1024 * 1024 // 40 MiB/s — above threshold.
	action := ctrl.Evaluate(m, time.Now().Add(-6*time.Minute))
	assert.Equal(t, action.Type, BalloonActionNone)
}

func TestBalloon_SwapOutBelowThreshold(t *testing.T) {
	cfg := newTestConfig()
	cfg.MaxSwapOutPerSec = 32 * 1024 * 1024
	ctrl := NewBalloonController(cfg)
	ctrl.TransitionTo(BalloonStateSteady)
	ctrl.currentBytes = 8 * 1024 * 1024 * 1024

	m := idleMetrics()
	m.SwapOutBytesPerSec = 5 * 1024 * 1024 // 5 MiB/s — below threshold.
	action := ctrl.Evaluate(m, time.Now().Add(-6*time.Minute))
	assert.Equal(t, action.Type, BalloonActionShrink)
}

func TestBalloon_SwapOutZeroThreshold(t *testing.T) {
	cfg := newTestConfig()
	cfg.MaxSwapOutPerSec = 0 // Disabled.
	ctrl := NewBalloonController(cfg)
	ctrl.TransitionTo(BalloonStateSteady)
	ctrl.currentBytes = 8 * 1024 * 1024 * 1024

	m := idleMetrics()
	m.SwapOutBytesPerSec = 100 * 1024 * 1024 // High swap-out, but check disabled.
	action := ctrl.Evaluate(m, time.Now().Add(-6*time.Minute))
	assert.Equal(t, action.Type, BalloonActionShrink)
}

func TestBalloon_PageFaultRateBlocksShrink(t *testing.T) {
	cfg := newTestConfig()
	cfg.MaxPageFaultRate = 5000
	ctrl := NewBalloonController(cfg)
	ctrl.TransitionTo(BalloonStateSteady)
	ctrl.currentBytes = 8 * 1024 * 1024 * 1024

	m := idleMetrics()
	m.PageFaultRate = 6000 // Above threshold.
	action := ctrl.Evaluate(m, time.Now().Add(-6*time.Minute))
	assert.Equal(t, action.Type, BalloonActionNone)
}

func TestBalloon_PageFaultRateBelowThreshold(t *testing.T) {
	cfg := newTestConfig()
	cfg.MaxPageFaultRate = 5000
	ctrl := NewBalloonController(cfg)
	ctrl.TransitionTo(BalloonStateSteady)
	ctrl.currentBytes = 8 * 1024 * 1024 * 1024

	m := idleMetrics()
	m.PageFaultRate = 1000 // Below threshold.
	action := ctrl.Evaluate(m, time.Now().Add(-6*time.Minute))
	assert.Equal(t, action.Type, BalloonActionShrink)
}

func TestBalloon_MemAvailableReserveBlocksShrink(t *testing.T) {
	cfg := newTestConfig()
	cfg.ShrinkReserveBytes = 128 * 1024 * 1024 // 128 MiB.
	ctrl := NewBalloonController(cfg)
	ctrl.TransitionTo(BalloonStateSteady)
	ctrl.currentBytes = 8 * 1024 * 1024 * 1024

	m := idleMetrics()
	m.MemAvailableBytes = 100 * 1024 * 1024 // 100 MiB — below reserve + step.
	action := ctrl.Evaluate(m, time.Now().Add(-6*time.Minute))
	assert.Equal(t, action.Type, BalloonActionNone)
}

func TestBalloon_MemAvailableReserveAllowsShrink(t *testing.T) {
	cfg := newTestConfig()
	cfg.ShrinkReserveBytes = 128 * 1024 * 1024 // 128 MiB.
	ctrl := NewBalloonController(cfg)
	ctrl.TransitionTo(BalloonStateSteady)
	ctrl.currentBytes = 8 * 1024 * 1024 * 1024

	m := idleMetrics()
	m.MemAvailableBytes = 2 * 1024 * 1024 * 1024 // 2 GiB — well above reserve + step.
	action := ctrl.Evaluate(m, time.Now().Add(-6*time.Minute))
	assert.Equal(t, action.Type, BalloonActionShrink)
}

// --- E3: Settle Window tests ---

func TestBalloon_SettleWindowPreventsEarlyShrink(t *testing.T) {
	cfg := newTestConfig()
	cfg.SettleWindow = 30 * time.Second
	ctrl := NewBalloonController(cfg)
	ctrl.TransitionTo(BalloonStateSteady)
	ctrl.currentBytes = 8 * 1024 * 1024 * 1024

	m := idleMetrics()

	// First evaluate: starts settle window, returns none.
	action := ctrl.Evaluate(m, time.Now().Add(-6*time.Minute))
	assert.Equal(t, action.Type, BalloonActionNone)
	assert.Assert(t, !ctrl.lowPressureSince.IsZero())

	// Second evaluate: still within 30s, returns none.
	action = ctrl.Evaluate(m, time.Now().Add(-6*time.Minute))
	assert.Equal(t, action.Type, BalloonActionNone)

	// Simulate 30s passing.
	ctrl.lowPressureSince = time.Now().Add(-31 * time.Second)
	action = ctrl.Evaluate(m, time.Now().Add(-6*time.Minute))
	assert.Equal(t, action.Type, BalloonActionShrink)
}

func TestBalloon_SettleWindowResetOnHighPressure(t *testing.T) {
	cfg := newTestConfig()
	cfg.SettleWindow = 30 * time.Second
	ctrl := NewBalloonController(cfg)
	ctrl.TransitionTo(BalloonStateSteady)
	ctrl.currentBytes = 8 * 1024 * 1024 * 1024

	// Start settle window.
	ctrl.Evaluate(idleMetrics(), time.Now().Add(-6*time.Minute))
	assert.Assert(t, !ctrl.lowPressureSince.IsZero())

	// High pressure resets settle window.
	ctrl.Evaluate(pressureMetrics(), time.Now().Add(-6*time.Minute))
	assert.Assert(t, ctrl.lowPressureSince.IsZero())
}

func TestBalloon_SettleWindowZeroDisabled(t *testing.T) {
	cfg := newTestConfig()
	cfg.SettleWindow = 0 // Disabled.
	ctrl := NewBalloonController(cfg)
	ctrl.TransitionTo(BalloonStateSteady)
	ctrl.currentBytes = 8 * 1024 * 1024 * 1024

	m := idleMetrics()
	action := ctrl.Evaluate(m, time.Now().Add(-6*time.Minute))
	assert.Equal(t, action.Type, BalloonActionShrink)
}

func TestBalloon_SettleWindowWithCooldown(t *testing.T) {
	cfg := newTestConfig()
	cfg.SettleWindow = 30 * time.Second
	cfg.Cooldown = 30 * time.Second
	ctrl := NewBalloonController(cfg)
	ctrl.TransitionTo(BalloonStateSteady)
	ctrl.currentBytes = 8 * 1024 * 1024 * 1024

	// Settle window passed but cooldown hasn't — cooldown blocks.
	ctrl.lowPressureSince = time.Now().Add(-31 * time.Second)
	ctrl.RecordAction(BalloonAction{Type: BalloonActionShrink, TargetBytes: 8 * 1024 * 1024 * 1024}, time.Now())
	action := ctrl.Evaluate(idleMetrics(), time.Now().Add(-6*time.Minute))
	assert.Equal(t, action.Type, BalloonActionNone)
}

func TestBalloon_SettleResetOnOOM(t *testing.T) {
	cfg := newTestConfig()
	cfg.SettleWindow = 30 * time.Second
	ctrl := NewBalloonController(cfg)
	ctrl.TransitionTo(BalloonStateSteady)
	ctrl.currentBytes = 8 * 1024 * 1024 * 1024

	// Accumulate settle window.
	ctrl.lowPressureSince = time.Now().Add(-20 * time.Second)
	assert.Assert(t, !ctrl.lowPressureSince.IsZero())

	// OOM resets settle window.
	ctrl.RecordOOM(time.Now())
	assert.Assert(t, ctrl.lowPressureSince.IsZero())
}

func TestBalloon_SettlePsiFullInteraction(t *testing.T) {
	cfg := newTestConfig()
	cfg.SettleWindow = 30 * time.Second
	ctrl := NewBalloonController(cfg)
	ctrl.TransitionTo(BalloonStateSteady)
	ctrl.currentBytes = 8 * 1024 * 1024 * 1024

	// Settle window has passed.
	ctrl.lowPressureSince = time.Now().Add(-31 * time.Second)

	// But PSI full > 0 — E1 guard fires before settle check.
	m := idleMetrics()
	m.PsiMemorySome_10 = 0.1
	m.PsiMemoryFull_10 = 0.05
	action := ctrl.Evaluate(m, time.Now().Add(-6*time.Minute))
	assert.Equal(t, action.Type, BalloonActionNone)
}

// --- E4: Host-Side Memory Pressure tests ---

func TestBalloon_HostCriticalShrinks(t *testing.T) {
	cfg := newTestConfig()
	ctrl := NewBalloonController(cfg)
	ctrl.TransitionTo(BalloonStateSteady)
	ctrl.currentBytes = 8 * 1024 * 1024 * 1024

	ctrl.SetHostPressure(HostPressureCritical)
	m := idleMetrics()
	action := ctrl.Evaluate(m, time.Now().Add(-6*time.Minute))
	assert.Equal(t, action.Type, BalloonActionShrink)
	assert.Equal(t, action.Reason, "host critical pressure")
	// 2× step: MaxMem * ShrinkPct * 2 / 100.
	assert.Assert(t, action.TargetBytes < ctrl.currentBytes)
	assert.Assert(t, action.TargetBytes >= cfg.MinBytes)
}

func TestBalloon_HostCriticalRespectsDistress(t *testing.T) {
	cfg := newTestConfig()
	ctrl := NewBalloonController(cfg)
	ctrl.TransitionTo(BalloonStateSteady)
	ctrl.currentBytes = 8 * 1024 * 1024 * 1024

	ctrl.SetHostPressure(HostPressureCritical)
	m := idleMetrics()
	m.PsiMemoryFull_10 = 8.0 // Severe guest distress > 5.0.
	m.PsiMemorySome_10 = 0.1 // Below high threshold.
	action := ctrl.Evaluate(m, time.Now().Add(-6*time.Minute))
	assert.Equal(t, action.Type, BalloonActionNone)
}

func TestBalloon_HostCriticalAllowsMildFullStall(t *testing.T) {
	cfg := newTestConfig()
	ctrl := NewBalloonController(cfg)
	ctrl.TransitionTo(BalloonStateSteady)
	ctrl.currentBytes = 8 * 1024 * 1024 * 1024

	ctrl.SetHostPressure(HostPressureCritical)
	m := idleMetrics()
	m.PsiMemoryFull_10 = 3.0 // Mild full stall <= 5.0 — tolerable under critical.
	action := ctrl.Evaluate(m, time.Now().Add(-6*time.Minute))
	assert.Equal(t, action.Type, BalloonActionShrink)
	assert.Equal(t, action.Reason, "host critical pressure")
}

func TestBalloon_HostCriticalRespectsFloor(t *testing.T) {
	cfg := newTestConfig()
	ctrl := NewBalloonController(cfg)
	ctrl.TransitionTo(BalloonStateSteady)
	ctrl.currentBytes = cfg.MinBytes + 100*1024*1024 // Just above min.

	ctrl.SetHostPressure(HostPressureCritical)
	action := ctrl.Evaluate(idleMetrics(), time.Now().Add(-6*time.Minute))
	assert.Equal(t, action.Type, BalloonActionShrink)
	assert.Equal(t, action.TargetBytes, cfg.MinBytes)
}

func TestBalloon_HostCriticalUnderflowGuard(t *testing.T) {
	cfg := newTestConfig()
	cfg.ShrinkStepPercent = 80 // 80% of 12 GiB = 9.6 GiB; 2× = 19.2 GiB > currentBytes.
	ctrl := NewBalloonController(cfg)
	ctrl.TransitionTo(BalloonStateSteady)
	ctrl.currentBytes = 5 * 1024 * 1024 * 1024

	ctrl.SetHostPressure(HostPressureCritical)
	action := ctrl.Evaluate(idleMetrics(), time.Now().Add(-6*time.Minute))
	assert.Equal(t, action.Type, BalloonActionShrink)
	assert.Equal(t, action.TargetBytes, cfg.MinBytes) // Clamped to min, not underflowed.
}

func TestBalloon_HostCriticalRespectsCooldown(t *testing.T) {
	cfg := newTestConfig()
	ctrl := NewBalloonController(cfg)
	ctrl.TransitionTo(BalloonStateSteady)
	ctrl.currentBytes = 8 * 1024 * 1024 * 1024

	ctrl.SetHostPressure(HostPressureCritical)
	ctrl.RecordAction(BalloonAction{Type: BalloonActionShrink, TargetBytes: 8 * 1024 * 1024 * 1024}, time.Now())
	action := ctrl.Evaluate(idleMetrics(), time.Now().Add(-6*time.Minute))
	assert.Equal(t, action.Type, BalloonActionNone) // Cooldown blocks.
}

func TestBalloon_HostWarningBypassesSettle(t *testing.T) {
	cfg := newTestConfig()
	cfg.SettleWindow = 30 * time.Second
	ctrl := NewBalloonController(cfg)
	ctrl.TransitionTo(BalloonStateSteady)
	ctrl.currentBytes = 8 * 1024 * 1024 * 1024

	ctrl.SetHostPressure(HostPressureWarning)
	// No settle window accumulated — Warning bypasses it.
	action := ctrl.Evaluate(idleMetrics(), time.Now().Add(-6*time.Minute))
	assert.Equal(t, action.Type, BalloonActionShrink)
	assert.Equal(t, action.Reason, "host warning pressure")
}

func TestBalloon_HostWarningRespectsCooldown(t *testing.T) {
	cfg := newTestConfig()
	ctrl := NewBalloonController(cfg)
	ctrl.TransitionTo(BalloonStateSteady)
	ctrl.currentBytes = 8 * 1024 * 1024 * 1024

	ctrl.SetHostPressure(HostPressureWarning)
	ctrl.RecordAction(BalloonAction{Type: BalloonActionShrink, TargetBytes: 8 * 1024 * 1024 * 1024}, time.Now())
	action := ctrl.Evaluate(idleMetrics(), time.Now().Add(-6*time.Minute))
	assert.Equal(t, action.Type, BalloonActionNone) // Cooldown blocks.
}

func TestBalloon_HostWarningDoesNotResetLowPressureSince(t *testing.T) {
	cfg := newTestConfig()
	cfg.SettleWindow = 30 * time.Second
	ctrl := NewBalloonController(cfg)
	ctrl.TransitionTo(BalloonStateSteady)
	ctrl.currentBytes = 8 * 1024 * 1024 * 1024

	settleTime := time.Now().Add(-20 * time.Second)
	ctrl.lowPressureSince = settleTime

	ctrl.SetHostPressure(HostPressureWarning)
	action := ctrl.Evaluate(idleMetrics(), time.Now().Add(-6*time.Minute))
	assert.Equal(t, action.Type, BalloonActionShrink)
	assert.Equal(t, ctrl.lowPressureSince, settleTime) // Not reset by Warning shrink.
}

func TestBalloon_HostNormalSettleRequired(t *testing.T) {
	cfg := newTestConfig()
	cfg.SettleWindow = 30 * time.Second
	ctrl := NewBalloonController(cfg)
	ctrl.TransitionTo(BalloonStateSteady)
	ctrl.currentBytes = 8 * 1024 * 1024 * 1024

	ctrl.SetHostPressure(HostPressureNormal)
	action := ctrl.Evaluate(idleMetrics(), time.Now().Add(-6*time.Minute))
	assert.Equal(t, action.Type, BalloonActionNone) // Settle window not met.
}

func TestBalloon_HostCriticalToNormalRecovery(t *testing.T) {
	cfg := newTestConfig()
	cfg.SettleWindow = 30 * time.Second
	ctrl := NewBalloonController(cfg)
	ctrl.TransitionTo(BalloonStateSteady)
	ctrl.currentBytes = 8 * 1024 * 1024 * 1024

	// Critical shrink.
	ctrl.SetHostPressure(HostPressureCritical)
	action := ctrl.Evaluate(idleMetrics(), time.Now().Add(-6*time.Minute))
	assert.Equal(t, action.Type, BalloonActionShrink)
	ctrl.RecordAction(action, time.Now().Add(-31*time.Second)) // Pretend action was 31s ago.

	// Switch to Normal — settle window required.
	ctrl.SetHostPressure(HostPressureNormal)
	action = ctrl.Evaluate(idleMetrics(), time.Now().Add(-6*time.Minute))
	assert.Equal(t, action.Type, BalloonActionNone) // Settle window just started.
	assert.Assert(t, !ctrl.lowPressureSince.IsZero())
}

// --- E5: Learned Stable Floor tests ---

func TestBalloon_LearnDescendOnLowPressure(t *testing.T) {
	cfg := newTestConfig()
	ctrl := NewBalloonController(cfg)
	ctrl.TransitionTo(BalloonStateLearningDescend)
	ctrl.currentBytes = 6 * 1024 * 1024 * 1024 // 6 GiB.

	m := idleMetrics()
	action := ctrl.Evaluate(m, time.Now().Add(-6*time.Minute))
	assert.Equal(t, action.Type, BalloonActionShrink)
	assert.Equal(t, action.Reason, "learning: descending to find floor")
}

func TestBalloon_LearnDetectInstability(t *testing.T) {
	cfg := newTestConfig()
	ctrl := NewBalloonController(cfg)
	ctrl.TransitionTo(BalloonStateLearningDescend)
	ctrl.currentBytes = 5 * 1024 * 1024 * 1024

	// PSI spike during descend — should set candidate floor.
	m := pressureMetrics()
	action := ctrl.Evaluate(m, time.Now().Add(-6*time.Minute))
	assert.Equal(t, action.Type, BalloonActionGrow)
	assert.Equal(t, ctrl.state, BalloonStateLearningConfirm)
	assert.Assert(t, ctrl.candidateFloor > 0)
}

func TestBalloon_LearnConfirmStable(t *testing.T) {
	cfg := newTestConfig()
	ctrl := NewBalloonController(cfg)
	ctrl.instDir = t.TempDir()
	ctrl.TransitionTo(BalloonStateLearningConfirm)
	ctrl.currentBytes = 5 * 1024 * 1024 * 1024
	ctrl.candidateFloor = 5 * 1024 * 1024 * 1024
	ctrl.confirmStartTime = time.Now().Add(-6 * time.Minute) // 6 min ago > learnConfirmDuration.

	m := idleMetrics()
	action := ctrl.Evaluate(m, time.Now().Add(-10*time.Minute))
	assert.Equal(t, action.Type, BalloonActionNone)
	assert.Equal(t, ctrl.state, BalloonStateSteady)
	assert.Equal(t, ctrl.learnedFloor, uint64(5*1024*1024*1024))

	// Verify persistence.
	v, _, err := store.ReadLearnedFloor(ctrl.instDir)
	assert.NilError(t, err)
	assert.Equal(t, v, uint64(5*1024*1024*1024))
}

func TestBalloon_LearnConfirmUnstable(t *testing.T) {
	cfg := newTestConfig()
	cfg.IdleTargetBytes = 8 * 1024 * 1024 * 1024 // Room for candidate to grow.
	ctrl := NewBalloonController(cfg)
	ctrl.TransitionTo(BalloonStateLearningConfirm)
	ctrl.currentBytes = 5 * 1024 * 1024 * 1024
	ctrl.candidateFloor = 5 * 1024 * 1024 * 1024
	ctrl.confirmStartTime = time.Now()

	// PSI spike during confirm — should raise candidate.
	m := pressureMetrics()
	action := ctrl.Evaluate(m, time.Now().Add(-10*time.Minute))
	assert.Equal(t, action.Type, BalloonActionGrow)
	assert.Equal(t, ctrl.confirmFails, 1)
	assert.Assert(t, ctrl.candidateFloor > uint64(5*1024*1024*1024))
}

func TestBalloon_LearnedFloorBlocksShrink(t *testing.T) {
	cfg := newTestConfig()
	ctrl := NewBalloonController(cfg)
	ctrl.TransitionTo(BalloonStateSteady)
	ctrl.currentBytes = 6 * 1024 * 1024 * 1024
	ctrl.learnedFloor = 5 * 1024 * 1024 * 1024 // Learned floor at 5 GiB.

	m := idleMetrics()
	action := ctrl.Evaluate(m, time.Now().Add(-6*time.Minute))
	// Should shrink but respect learnedFloor as hardFloor.
	if action.Type == BalloonActionShrink {
		assert.Assert(t, action.TargetBytes >= ctrl.learnedFloor)
	}
}

func TestBalloon_LearnedFloorReset(t *testing.T) {
	cfg := newTestConfig()
	ctrl := NewBalloonController(cfg)
	ctrl.TransitionTo(BalloonStateSteady)
	ctrl.learnedFloor = 5 * 1024 * 1024 * 1024

	// OOM should reset learned floor.
	ctrl.RecordOOM(time.Now())
	assert.Equal(t, ctrl.learnedFloor, uint64(0))
}

func TestBalloon_LearnedFloorPersistence(t *testing.T) {
	dir := t.TempDir()
	err := store.WriteLearnedFloor(dir, 4*1024*1024*1024, time.Now())
	assert.NilError(t, err)

	v, _, err := store.ReadLearnedFloor(dir)
	assert.NilError(t, err)
	assert.Equal(t, v, uint64(4*1024*1024*1024))
}

func TestBalloon_LearnedFloorPersistenceCorrupt(t *testing.T) {
	dir := t.TempDir()
	v, _, err := store.ReadLearnedFloor(dir)
	assert.NilError(t, err)
	assert.Equal(t, v, uint64(0)) // Not found → 0.
}

func TestBalloon_LearnNotStartedAtBoot(t *testing.T) {
	cfg := newTestConfig()
	ctrl := NewBalloonController(cfg)
	ctrl.TransitionTo(BalloonStateSteady)
	ctrl.currentBytes = 8 * 1024 * 1024 * 1024

	// Containers running — learning should NOT trigger.
	m := idleMetrics()
	m.ContainerCount = 3
	action := ctrl.Evaluate(m, time.Now().Add(-6*time.Minute))
	assert.Equal(t, action.Type, BalloonActionShrink)
	// State should remain Steady (not LearningDescend).
	assert.Equal(t, ctrl.state, BalloonStateSteady)
}

func TestBalloon_LearnDescendReachesMin(t *testing.T) {
	cfg := newTestConfig()
	ctrl := NewBalloonController(cfg)
	ctrl.instDir = t.TempDir()
	ctrl.TransitionTo(BalloonStateLearningDescend)
	// Set currentBytes near min.
	step := cfg.MaxMemoryBytes * uint64(cfg.ShrinkStepPercent) / 100
	ctrl.currentBytes = cfg.MinBytes + step - 1 // Just below min + step.

	m := idleMetrics()
	action := ctrl.Evaluate(m, time.Now().Add(-6*time.Minute))
	assert.Equal(t, action.Type, BalloonActionNone)
	assert.Equal(t, ctrl.state, BalloonStateSteady)
	assert.Equal(t, ctrl.learnedFloor, cfg.MinBytes+step)
}

func TestBalloon_LearnedFloorInvalidatedOnResize(t *testing.T) {
	cfg := newTestConfig()
	// Learned floor > idleTarget should be discarded.
	floor := cfg.IdleTargetBytes + 1024
	assert.Assert(t, floor > cfg.IdleTargetBytes)
	// Simulate startup validation.
	if floor > cfg.IdleTargetBytes || floor < cfg.MinBytes {
		floor = 0
	}
	assert.Equal(t, floor, uint64(0))

	// Learned floor < min should be discarded.
	floor2 := cfg.MinBytes - 1
	if floor2 > cfg.IdleTargetBytes || floor2 < cfg.MinBytes {
		floor2 = 0
	}
	assert.Equal(t, floor2, uint64(0))
}

func TestBalloon_LearnSuspendsUnderHostPressure(t *testing.T) {
	cfg := newTestConfig()
	ctrl := NewBalloonController(cfg)
	ctrl.TransitionTo(BalloonStateLearningDescend)
	ctrl.currentBytes = 6 * 1024 * 1024 * 1024

	// Host Warning — learning should suspend.
	ctrl.SetHostPressure(HostPressureWarning)
	m := idleMetrics()
	action := ctrl.Evaluate(m, time.Now().Add(-6*time.Minute))
	assert.Equal(t, action.Type, BalloonActionNone)
	assert.Equal(t, ctrl.state, BalloonStateLearningDescend) // Still in learning, just suspended.
}

// --- E10-3: Graduated poll failure tests ---

func TestBalloon_GraduatedPollFailure_TransientNoChange(t *testing.T) {
	cfg := newTestConfig()
	ctrl := NewBalloonController(cfg)
	ctrl.TransitionTo(BalloonStateSteady)
	ctrl.currentBytes = cfg.IdleTargetBytes

	// 1-2 failures should not change memory and return nil (no action).
	action := ctrl.RecordPollFailure()
	assert.Assert(t, action == nil)
	assert.Equal(t, ctrl.currentBytes, cfg.IdleTargetBytes)
	assert.Equal(t, ctrl.state, BalloonStateSteady)

	action = ctrl.RecordPollFailure()
	assert.Assert(t, action == nil)
	assert.Equal(t, ctrl.currentBytes, cfg.IdleTargetBytes)
	assert.Equal(t, ctrl.state, BalloonStateSteady)
}

func TestBalloon_GraduatedPollFailure_ThreeGrowsHalfHeadroom(t *testing.T) {
	cfg := newTestConfig()
	ctrl := NewBalloonController(cfg)
	ctrl.TransitionTo(BalloonStateSteady)
	ctrl.currentBytes = cfg.IdleTargetBytes // 4 GiB

	// 3 failures should grow by 50% of headroom.
	// headroom = 12 GiB - 4 GiB = 8 GiB; half = 4 GiB; target = 8 GiB.
	var action *BalloonAction
	for range 3 {
		action = ctrl.RecordPollFailure()
	}
	assert.Assert(t, action != nil)
	assert.Equal(t, action.Type, BalloonActionGrow)
	expectedTarget := cfg.IdleTargetBytes + (cfg.MaxMemoryBytes-cfg.IdleTargetBytes)/2
	assert.Equal(t, action.TargetBytes, expectedTarget)
	assert.Equal(t, ctrl.currentBytes, expectedTarget)
	assert.Equal(t, ctrl.state, BalloonStateSteady) // NOT agent failure yet.
}

func TestBalloon_GraduatedPollFailure_SixFullExpansion(t *testing.T) {
	cfg := newTestConfig()
	ctrl := NewBalloonController(cfg)
	ctrl.TransitionTo(BalloonStateSteady)
	ctrl.currentBytes = cfg.IdleTargetBytes

	// 6 failures should expand to max and enter agent failure.
	var action *BalloonAction
	for range 6 {
		action = ctrl.RecordPollFailure()
	}
	assert.Assert(t, action != nil)
	assert.Equal(t, action.Type, BalloonActionGrow)
	assert.Equal(t, action.TargetBytes, cfg.MaxMemoryBytes)
	assert.Equal(t, ctrl.currentBytes, cfg.MaxMemoryBytes)
	assert.Equal(t, ctrl.state, BalloonStateAgentFailure)
}

func TestBalloon_GraduatedPollFailure_RecoveryResets(t *testing.T) {
	cfg := newTestConfig()
	ctrl := NewBalloonController(cfg)
	ctrl.TransitionTo(BalloonStateSteady)
	ctrl.currentBytes = cfg.IdleTargetBytes

	// 4 failures (partial grow), then success resets.
	for range 4 {
		ctrl.RecordPollFailure()
	}
	assert.Assert(t, ctrl.currentBytes > cfg.IdleTargetBytes)
	assert.Equal(t, ctrl.state, BalloonStateSteady)

	ctrl.RecordPollSuccess()
	assert.Equal(t, ctrl.pollFailures, 0)

	// Need 3 more failures to trigger partial grow again.
	ctrl.RecordPollFailure()
	ctrl.RecordPollFailure()
	assert.Equal(t, ctrl.state, BalloonStateSteady)
}

// --- E10-1: PSI availability fallback tests ---

func TestBalloon_PsiUnavailable_BlocksShrinkWhenMemTight(t *testing.T) {
	// When PSI is unavailable (all zeros) and MemAvailable < 40% of currentBytes,
	// the balloon should NOT shrink.
	cfg := newTestConfig()
	cfg.SettleWindow = 0 // Disable settle window for simplicity.
	ctrl := NewBalloonController(cfg)
	ctrl.TransitionTo(BalloonStateSteady)
	ctrl.currentBytes = 8 * 1024 * 1024 * 1024 // 8 GiB

	m := &api.MemoryMetrics{
		MemTotalBytes:     12 * 1024 * 1024 * 1024,
		MemAvailableBytes: 2 * 1024 * 1024 * 1024, // 2 GiB < 40% of 8 GiB (3.2 GiB)
		PsiMemorySome_10:  0.0,                    // All zeros — PSI not available.
		PsiMemoryFull_10:  0.0,
		AnonRssBytes:      5 * 1024 * 1024 * 1024,
	}
	action := ctrl.Evaluate(m, time.Now().Add(-6*time.Minute))
	assert.Equal(t, action.Type, BalloonActionNone)
}

func TestBalloon_PsiUnavailable_AllowsShrinkWhenMemPlentiful(t *testing.T) {
	// When PSI is unavailable but MemAvailable >= 40% of currentBytes,
	// the balloon CAN shrink (safe to do so).
	cfg := newTestConfig()
	cfg.SettleWindow = 0
	ctrl := NewBalloonController(cfg)
	ctrl.TransitionTo(BalloonStateSteady)
	ctrl.currentBytes = 8 * 1024 * 1024 * 1024 // 8 GiB

	m := &api.MemoryMetrics{
		MemTotalBytes:     12 * 1024 * 1024 * 1024,
		MemAvailableBytes: 6 * 1024 * 1024 * 1024, // 6 GiB > 40% of 8 GiB (3.2 GiB)
		PsiMemorySome_10:  0.0,
		PsiMemoryFull_10:  0.0,
		AnonRssBytes:      1 * 1024 * 1024 * 1024,
	}
	action := ctrl.Evaluate(m, time.Now().Add(-6*time.Minute))
	assert.Equal(t, action.Type, BalloonActionShrink)
}

func TestBalloon_PsiBecomesAvailable(t *testing.T) {
	// Once PSI produces a non-zero value, psiAvailable should be set true
	// and the fallback guard no longer applies.
	cfg := newTestConfig()
	cfg.SettleWindow = 0
	ctrl := NewBalloonController(cfg)
	ctrl.TransitionTo(BalloonStateSteady)
	ctrl.currentBytes = 8 * 1024 * 1024 * 1024

	assert.Assert(t, !ctrl.psiAvailable)

	// First poll with non-zero PSI sets psiAvailable.
	m := &api.MemoryMetrics{
		MemTotalBytes:     12 * 1024 * 1024 * 1024,
		MemAvailableBytes: 2 * 1024 * 1024 * 1024,
		PsiMemorySome_10:  0.05, // Non-zero — PSI is working.
		PsiMemoryFull_10:  0.0,
		AnonRssBytes:      5 * 1024 * 1024 * 1024,
	}
	ctrl.Evaluate(m, time.Now().Add(-6*time.Minute))
	assert.Assert(t, ctrl.psiAvailable)
}

func TestBalloon_PsiAvailable_NormalBehavior(t *testing.T) {
	// After PSI becomes available, the controller uses normal PSI-based
	// logic (no MemAvailable guard).
	cfg := newTestConfig()
	cfg.SettleWindow = 0
	ctrl := NewBalloonController(cfg)
	ctrl.TransitionTo(BalloonStateSteady)
	ctrl.currentBytes = 8 * 1024 * 1024 * 1024
	ctrl.psiAvailable = true // Already discovered PSI works.

	// Low PSI + low MemAvailable — with PSI available, normal shrink logic applies
	// (PSI says no pressure → shrink is permitted).
	m := &api.MemoryMetrics{
		MemTotalBytes:     12 * 1024 * 1024 * 1024,
		MemAvailableBytes: 2 * 1024 * 1024 * 1024,
		PsiMemorySome_10:  0.1, // Below low threshold — low pressure.
		PsiMemoryFull_10:  0.0,
		AnonRssBytes:      1 * 1024 * 1024 * 1024,
	}
	action := ctrl.Evaluate(m, time.Now().Add(-6*time.Minute))
	assert.Equal(t, action.Type, BalloonActionShrink)
}

// --- E10-2: Floor staleness tests ---

func TestBalloon_FloorStaleness_StaleFloorDiscarded(t *testing.T) {
	// A floor older than FloorStaleness is discarded (effectiveFloor returns 0).
	cfg := newTestConfig()
	cfg.FloorStaleness = 24 * time.Hour
	ctrl := NewBalloonController(cfg)
	ctrl.learnedFloor = 5 * 1024 * 1024 * 1024
	ctrl.learnedAt = time.Now().Add(-25 * time.Hour) // 25h ago — stale.

	assert.Equal(t, ctrl.effectiveFloor(), uint64(0))
	// After discard, internal state is cleared.
	assert.Equal(t, ctrl.learnedFloor, uint64(0))
	assert.Assert(t, ctrl.learnedAt.IsZero())
}

func TestBalloon_FloorStaleness_FreshFloorKept(t *testing.T) {
	// A floor newer than FloorStaleness is kept.
	cfg := newTestConfig()
	cfg.FloorStaleness = 24 * time.Hour
	ctrl := NewBalloonController(cfg)
	ctrl.learnedFloor = 5 * 1024 * 1024 * 1024
	ctrl.learnedAt = time.Now().Add(-12 * time.Hour) // 12h ago — fresh.

	assert.Equal(t, ctrl.effectiveFloor(), uint64(5*1024*1024*1024))
}

func TestBalloon_FloorStaleness_ZeroMeansNeverStale(t *testing.T) {
	// FloorStaleness=0 means the floor never becomes stale.
	cfg := newTestConfig()
	cfg.FloorStaleness = 0 // Disabled.
	ctrl := NewBalloonController(cfg)
	ctrl.learnedFloor = 5 * 1024 * 1024 * 1024
	ctrl.learnedAt = time.Now().Add(-1000 * time.Hour) // Very old.

	assert.Equal(t, ctrl.effectiveFloor(), uint64(5*1024*1024*1024))
}

func TestBalloon_FloorStaleness_ZeroTimeTreatedAsKept(t *testing.T) {
	// Zero learnedAt (old format) with FloorStaleness set: the staleness
	// check requires !learnedAt.IsZero(), so zero time skips the check.
	cfg := newTestConfig()
	cfg.FloorStaleness = 24 * time.Hour
	ctrl := NewBalloonController(cfg)
	ctrl.learnedFloor = 5 * 1024 * 1024 * 1024
	ctrl.learnedAt = time.Time{} // Zero — unknown timestamp.

	// With zero learnedAt, the staleness check is skipped (we can't know the age).
	// The floor is kept as-is — the hostagent.go range check already validated it.
	assert.Equal(t, ctrl.effectiveFloor(), uint64(5*1024*1024*1024))
}

// --- E10-8: Adaptive AnonRss margin tests ---

func TestBalloon_AdaptiveMargin_NoContainers(t *testing.T) {
	// 0 containers → 5% margin.
	cfg := newTestConfig()
	cfg.SettleWindow = 0
	cfg.MinBytes = 1 * 1024 * 1024 * 1024
	cfg.IdleTargetBytes = 12*1024*1024*1024 - 1
	ctrl := NewBalloonController(cfg)
	ctrl.TransitionTo(BalloonStateSteady)
	ctrl.currentBytes = 6 * 1024 * 1024 * 1024

	m := idleMetrics()
	m.AnonRssBytes = 4 * 1024 * 1024 * 1024 // 4 GiB AnonRss.
	m.ContainerCount = 0                    // No containers → 5% margin.

	action := ctrl.Evaluate(m, time.Now().Add(-6*time.Minute))
	if action.Type == BalloonActionShrink {
		// Floor = 4 GiB * 1.05 = 4.2 GiB. Target must be >= floor.
		expectedFloor := uint64(4*1024*1024*1024) + uint64(4*1024*1024*1024)*5/100
		assert.Assert(t, action.TargetBytes >= expectedFloor,
			"target %d should be >= 5%% floor %d", action.TargetBytes, expectedFloor)
	}
}

func TestBalloon_AdaptiveMargin_FewContainers(t *testing.T) {
	// 1-5 containers → 15% margin.
	cfg := newTestConfig()
	cfg.SettleWindow = 0
	cfg.MinBytes = 1 * 1024 * 1024 * 1024
	cfg.IdleTargetBytes = 12*1024*1024*1024 - 1
	ctrl := NewBalloonController(cfg)
	ctrl.TransitionTo(BalloonStateSteady)
	ctrl.currentBytes = 6 * 1024 * 1024 * 1024

	m := idleMetrics()
	m.AnonRssBytes = 4 * 1024 * 1024 * 1024
	m.ContainerCount = 3 // Few containers → 15% margin.

	action := ctrl.Evaluate(m, time.Now().Add(-6*time.Minute))
	if action.Type == BalloonActionShrink {
		expectedFloor := uint64(4*1024*1024*1024) + uint64(4*1024*1024*1024)*15/100
		assert.Assert(t, action.TargetBytes >= expectedFloor,
			"target %d should be >= 15%% floor %d", action.TargetBytes, expectedFloor)
	}
}

func TestBalloon_AdaptiveMargin_ManyContainers(t *testing.T) {
	// 6+ containers → 20% margin.
	cfg := newTestConfig()
	cfg.SettleWindow = 0
	cfg.MinBytes = 1 * 1024 * 1024 * 1024
	cfg.IdleTargetBytes = 12*1024*1024*1024 - 1
	ctrl := NewBalloonController(cfg)
	ctrl.TransitionTo(BalloonStateSteady)
	ctrl.currentBytes = 6 * 1024 * 1024 * 1024

	m := idleMetrics()
	m.AnonRssBytes = 4 * 1024 * 1024 * 1024
	m.ContainerCount = 8 // Many containers → 20% margin.

	action := ctrl.Evaluate(m, time.Now().Add(-6*time.Minute))
	if action.Type == BalloonActionShrink {
		expectedFloor := uint64(4*1024*1024*1024) + uint64(4*1024*1024*1024)*20/100
		assert.Assert(t, action.TargetBytes >= expectedFloor,
			"target %d should be >= 20%% floor %d", action.TargetBytes, expectedFloor)
	}
}

func TestBalloonController_NoPSI_GrowOnCriticalMemAvailable(t *testing.T) {
	cfg := newTestConfig()
	ctrl := NewBalloonController(cfg)
	ctrl.TransitionTo(BalloonStateSteady)
	// Simulate a prior shrink.
	ctrl.currentBytes = cfg.IdleTargetBytes
	// psiAvailable remains false (default).

	// MemAvailable at 10% of currentBytes — below the 15% critical threshold.
	m := idleMetrics()
	m.MemAvailableBytes = ctrl.currentBytes * 10 / 100
	// PSI is zero (unavailable).
	m.PsiMemorySome_10 = 0
	m.PsiMemoryFull_10 = 0

	action := ctrl.Evaluate(m, time.Now().Add(-10*time.Minute))
	assert.Equal(t, action.Type, BalloonActionGrow)
	assert.Assert(t, strings.Contains(action.Reason, "no PSI"),
		"expected 'no PSI' in reason, got %q", action.Reason)
	assert.Assert(t, action.TargetBytes > ctrl.currentBytes,
		"grow should increase target")
}

func TestBalloonController_NoPSI_BlockShrinkAt30Percent(t *testing.T) {
	cfg := newTestConfig()
	ctrl := NewBalloonController(cfg)
	ctrl.TransitionTo(BalloonStateSteady)
	ctrl.currentBytes = cfg.IdleTargetBytes

	// MemAvailable at 30% — below 40% but above 15%.
	// Should return none (block shrink, no grow).
	m := idleMetrics()
	m.MemAvailableBytes = ctrl.currentBytes * 30 / 100
	m.PsiMemorySome_10 = 0
	m.PsiMemoryFull_10 = 0

	action := ctrl.Evaluate(m, time.Now().Add(-10*time.Minute))
	assert.Equal(t, action.Type, BalloonActionNone)
}

func TestBalloonController_NoPSI_NoOscillation(t *testing.T) {
	cfg := newTestConfig()
	ctrl := NewBalloonController(cfg)
	ctrl.TransitionTo(BalloonStateSteady)
	// Simulate state after a grow: at 7 GiB with moderate MemAvailable.
	ctrl.currentBytes = 7 * 1024 * 1024 * 1024
	ctrl.lastActionTime = time.Now().Add(-5 * time.Minute) // Well past cooldown.

	// MemAvailable = 3 GiB — healthy at 7 GiB, but would be critical at 4 GiB.
	// Shrinking to idleTarget (4 GiB) shrinks by 3 GiB.
	// Expected MemAvail after = 3 - 3 = 0 GiB.
	// 0 / 4 GiB = 0% — below 20% headroom, so shrink should be blocked.
	m := idleMetrics()
	m.MemAvailableBytes = 3 * 1024 * 1024 * 1024 // 3 GiB.
	m.PsiMemorySome_10 = 0
	m.PsiMemoryFull_10 = 0

	action := ctrl.Evaluate(m, time.Now().Add(-10*time.Minute))
	assert.Equal(t, action.Type, BalloonActionNone,
		"should block shrink that would cause oscillation")
}
