// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package hostagent

import (
	"sync"
	"time"

	"github.com/docker/go-units"
	"github.com/sirupsen/logrus"

	"github.com/lima-vm/lima/v2/pkg/guestagent/api"
	"github.com/lima-vm/lima/v2/pkg/store"
)

const (
	learnConfirmDuration = 5 * time.Minute // How long to confirm a candidate floor.
	learnMaxConfirmFails = 3               // Max failed confirmations before fallback.
)

// BalloonState represents the state machine state of the balloon controller.
type BalloonState string

const (
	BalloonStateBootstrap       BalloonState = "bootstrap"
	BalloonStateSteady          BalloonState = "steady"
	BalloonStateAgentFailure    BalloonState = "agent-failure"
	BalloonStateCircuitBreaker  BalloonState = "circuit-breaker"
	BalloonStateShutdown        BalloonState = "shutdown"
	BalloonStateLearningDescend BalloonState = "learning-descend"
	BalloonStateLearningConfirm BalloonState = "learning-confirm"
)

// BalloonActionType describes what the controller wants to do.
type BalloonActionType string

const (
	BalloonActionNone   BalloonActionType = "none"
	BalloonActionGrow   BalloonActionType = "grow"
	BalloonActionShrink BalloonActionType = "shrink"
)

// BalloonAction is the output of the controller's evaluation.
type BalloonAction struct {
	Type        BalloonActionType
	TargetBytes uint64
	Reason      string
}

// BalloonConfig holds the configuration for the balloon controller.
type BalloonConfig struct {
	MaxMemoryBytes        uint64
	MinBytes              uint64
	IdleTargetBytes       uint64
	GrowStepPercent       int
	ShrinkStepPercent     int
	HighPressureThreshold float64
	LowPressureThreshold  float64
	Cooldown              time.Duration
	IdleGracePeriod       time.Duration
	MaxSwapInPerSec       uint64
	MaxSwapOutPerSec      uint64        // Swap-out rate that blocks shrinking (bytes/sec).
	MaxPageFaultRate      uint64        // Page-fault rate that blocks shrinking (faults/sec).
	ShrinkReserveBytes    uint64        // Minimum MemAvailable margin required before shrinking (bytes).
	SettleWindow          time.Duration // Sustained low pressure required before shrink.
	MaxContainerCPU       float64
	MaxContainerIO        uint64
	FloorStaleness        time.Duration // Max age of learned floor before re-learning (0 = never stale).
}

// BalloonController implements the balloon state machine.
// It is NOT safe for concurrent use except for SetHostPressure, which is
// protected by a mutex. All other methods must be called from a single goroutine
// (typically the balloon polling loop in hostagent).
type BalloonController struct {
	cfg          BalloonConfig
	state        BalloonState
	currentBytes uint64

	mu               sync.Mutex // Protects hostPressure for concurrent access.
	hostPressure     HostPressure
	lastActionTime   time.Time
	lowPressureSince time.Time // When sustained low pressure started (zero = not settled).
	pollFailures     int
	oomTimes         []time.Time
	circuitBreakerT  time.Time

	// E5: Learned stable floor fields.
	learnedFloor     uint64    // Discovered stable floor (bytes); 0 = not yet learned.
	learnedAt        time.Time // When the floor was learned (zero = unknown/stale).
	candidateFloor   uint64    // Candidate floor being confirmed.
	confirmStartTime time.Time // When confirmation started.
	confirmFails     int       // Number of failed confirmations at current candidate.
	instDir          string    // Instance directory for persisting learned floor.

	// E10-1: PSI availability tracking.
	psiAvailable bool // Set true on first non-zero PSI reading.
}

// NewBalloonController creates a controller starting in bootstrap state at max memory.
func NewBalloonController(cfg BalloonConfig) *BalloonController {
	return &BalloonController{
		cfg:          cfg,
		state:        BalloonStateBootstrap,
		currentBytes: cfg.MaxMemoryBytes,
	}
}

// effectiveFloor returns the learned floor if it is still fresh, or 0 if stale/unset.
func (c *BalloonController) effectiveFloor() uint64 {
	if c.learnedFloor == 0 {
		return 0
	}
	if c.cfg.FloorStaleness > 0 && !c.learnedAt.IsZero() && time.Since(c.learnedAt) > c.cfg.FloorStaleness {
		logrus.Infof("balloon: learned floor %d bytes is stale (age %s > %s), discarding",
			c.learnedFloor, time.Since(c.learnedAt).Round(time.Second), c.cfg.FloorStaleness)
		c.learnedFloor = 0
		c.learnedAt = time.Time{}
		return 0
	}
	return c.learnedFloor
}

// TransitionTo changes the controller state.
func (c *BalloonController) TransitionTo(state BalloonState) {
	logrus.Debugf("balloon: state %s -> %s", c.state, state)
	c.state = state
}

// SetHostPressure updates the host memory pressure level (thread-safe).
func (c *BalloonController) SetHostPressure(p HostPressure) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.hostPressure = p
}

// RecordAction records that an action was taken at the given time.
func (c *BalloonController) RecordAction(action BalloonAction, t time.Time) {
	c.lastActionTime = t
	if action.Type != BalloonActionNone {
		c.currentBytes = action.TargetBytes
	}
}

// RecordOOM records an OOM event for circuit breaker tracking.
func (c *BalloonController) RecordOOM(t time.Time) {
	c.lowPressureSince = time.Time{} // Reset settle window on OOM.
	c.learnedFloor = 0               // Reset learned floor — workload may have changed.
	c.oomTimes = append(c.oomTimes, t)
	// Keep only OOMs in the last 10 minutes.
	cutoff := t.Add(-10 * time.Minute)
	filtered := c.oomTimes[:0]
	for _, oomT := range c.oomTimes {
		if oomT.After(cutoff) {
			filtered = append(filtered, oomT)
		}
	}
	c.oomTimes = filtered

	// Circuit breaker: 3+ OOMs in 10 minutes.
	if len(c.oomTimes) >= 3 {
		c.state = BalloonStateCircuitBreaker
		c.circuitBreakerT = t
		c.currentBytes = c.cfg.MaxMemoryBytes
		logrus.Warnf("balloon: circuit breaker triggered (%d OOMs in 10 min), locked at max", len(c.oomTimes))
	}
}

// RecordPollFailure records a failed metrics poll with graduated response.
// 1-2 failures: transient, no action. 3-5: grow by 50% of headroom.
// 6+: full expansion to max, enter agent failure state.
// Returns a non-nil action when memory should be expanded.
func (c *BalloonController) RecordPollFailure() *BalloonAction {
	c.pollFailures++
	switch {
	case c.pollFailures >= 6:
		// 60s of failures — full expansion, enter agent failure state.
		if c.state != BalloonStateAgentFailure {
			c.TransitionTo(BalloonStateAgentFailure)
			c.currentBytes = c.cfg.MaxMemoryBytes
			logrus.Warn("balloon: 6 poll failures, expanding to max memory")
			return &BalloonAction{Type: BalloonActionGrow, TargetBytes: c.currentBytes, Reason: "agent failure"}
		}
	case c.pollFailures >= 3:
		// 30s of failures — grow by 50% of remaining headroom.
		headroom := c.cfg.MaxMemoryBytes - c.currentBytes
		if headroom > 0 {
			grow := headroom / 2
			c.currentBytes = min(c.currentBytes+grow, c.cfg.MaxMemoryBytes)
			logrus.Warnf("balloon: %d poll failures, growing to %s",
				c.pollFailures, units.BytesSize(float64(c.currentBytes)))
			return &BalloonAction{Type: BalloonActionGrow, TargetBytes: c.currentBytes, Reason: "poll failure recovery"}
		}
	}
	// 1-2 failures: do nothing (transient).
	return nil
}

// RecordPollSuccess resets the poll failure counter and recovers from agent failure.
func (c *BalloonController) RecordPollSuccess() {
	if c.pollFailures > 0 {
		c.pollFailures = 0
		if c.state == BalloonStateAgentFailure {
			c.TransitionTo(BalloonStateSteady)
			logrus.Info("balloon: agent recovered, resuming steady state")
		}
	}
}

// PrepareShutdown grows memory to max before VM stop.
func (c *BalloonController) PrepareShutdown() BalloonAction {
	c.state = BalloonStateShutdown
	return BalloonAction{
		Type:        BalloonActionGrow,
		TargetBytes: c.cfg.MaxMemoryBytes,
		Reason:      "graceful shutdown",
	}
}

// Evaluate examines metrics and returns the balloon action to take.
// bootTime is when the VM started (used for idle grace period).
func (c *BalloonController) Evaluate(m *api.MemoryMetrics, bootTime time.Time) BalloonAction {
	none := BalloonAction{Type: BalloonActionNone, TargetBytes: c.currentBytes}

	if m == nil {
		return none
	}

	switch c.state {
	case BalloonStateBootstrap:
		return none
	case BalloonStateCircuitBreaker:
		// Stay at max until circuit breaker timeout (30 min).
		if time.Since(c.circuitBreakerT) > 30*time.Minute {
			c.state = BalloonStateSteady
			c.oomTimes = nil
			logrus.Info("balloon: circuit breaker reset")
		}
		return none
	case BalloonStateAgentFailure:
		return none
	case BalloonStateShutdown:
		return none
	case BalloonStateLearningDescend:
		return c.evaluateLearningDescend(m, none)
	case BalloonStateLearningConfirm:
		return c.evaluateLearningConfirm(m, none)
	}

	// OOM handling — immediate grow, no cooldown.
	if m.OomDetected {
		target := min(c.currentBytes+c.currentBytes/5, c.cfg.MaxMemoryBytes)
		c.RecordOOM(time.Now())
		return BalloonAction{
			Type:        BalloonActionGrow,
			TargetBytes: target,
			Reason:      "OOM detected",
		}
	}

	// E10-1: Track PSI availability. Set true on first non-zero reading.
	if !c.psiAvailable {
		if m.PsiMemorySome_10 > 0 || m.PsiMemoryFull_10 > 0 {
			c.psiAvailable = true
		}
	}

	// E10-1: PSI unavailable guard — prevent aggressive shrinking without PSI data.
	// When PSI returns all zeros (disabled kernel or first boot), fall back to
	// MemAvailable-only heuristic: block shrink if MemAvailable < 40% of currentBytes,
	// and trigger grow if MemAvailable < 15% (critical pressure without PSI).
	if !c.psiAvailable {
		if m.MemAvailableBytes < c.currentBytes*15/100 {
			// Critical: MemAvailable < 15% of current balloon — grow immediately.
			// Use 3× cooldown to avoid oscillation: after a shrink, the kernel needs
			// time to stabilize MemAvailable before we evaluate pressure again.
			noPSICooldown := c.cfg.Cooldown * 3
			if !c.lastActionTime.IsZero() && time.Since(c.lastActionTime) < noPSICooldown {
				return none
			}
			step := c.cfg.MaxMemoryBytes * uint64(c.cfg.GrowStepPercent) / 100
			target := min(c.currentBytes+step, c.cfg.MaxMemoryBytes)
			if target > c.currentBytes {
				return BalloonAction{
					Type:        BalloonActionGrow,
					TargetBytes: target,
					Reason:      "low MemAvailable (no PSI)",
				}
			}
			return none // Already at max, nothing to do.
		}
		if m.MemAvailableBytes < c.currentBytes*40/100 {
			return none // Block shrink but no grow needed yet.
		}
	}

	// E4: Host pressure modifiers — checked BEFORE E1 guest distress.
	c.mu.Lock()
	hp := c.hostPressure
	c.mu.Unlock()
	switch hp {
	case HostPressureCritical:
		if m.PsiMemoryFull_10 > 5.0 {
			// Severe guest distress even by host-critical standards — hold steady.
			if m.PsiMemorySome_10 < c.cfg.HighPressureThreshold {
				return none
			}
			// some >= high → fall through to grow path.
		} else if time.Since(c.lastActionTime) >= c.cfg.Cooldown {
			// Guest not severely distressed — aggressive 2× shrink.
			step := c.cfg.MaxMemoryBytes * uint64(c.cfg.ShrinkStepPercent) * 2 / 100
			var target uint64
			if c.currentBytes > step {
				target = max(c.cfg.MinBytes, c.currentBytes-step)
			} else {
				target = c.cfg.MinBytes
			}
			return BalloonAction{
				Type: BalloonActionShrink, TargetBytes: target,
				Reason: "host critical pressure",
			}
		}
	case HostPressureWarning:
		// Bypass settle window and E2 guards; cooldown still applies.
		guestDistressed := m.PsiMemoryFull_10 > 0
		if !guestDistressed && m.PsiMemorySome_10 < c.cfg.HighPressureThreshold {
			if time.Since(c.lastActionTime) >= c.cfg.Cooldown {
				step := c.cfg.MaxMemoryBytes * uint64(c.cfg.ShrinkStepPercent) / 100
				var target uint64
				if c.currentBytes > step {
					target = max(c.cfg.MinBytes, c.currentBytes-step)
				} else {
					target = c.cfg.MinBytes
				}
				return BalloonAction{
					Type: BalloonActionShrink, TargetBytes: target,
					Reason: "host warning pressure",
				}
			}
		}
	case HostPressureNormal:
		// Existing behavior — settle window and cooldown apply.
	}

	// E1: Guest distress detection — only under Normal/Warning.
	// Under Critical, already handled above with relaxed full > 5.0 threshold.
	if hp == HostPressureNormal || hp == HostPressureWarning {
		if m.PsiMemoryFull_10 > 0 {
			if m.PsiMemorySome_10 < c.cfg.HighPressureThreshold {
				return none
			}
		}
	}

	// E3: Reset settle window when pressure rises above low threshold.
	if m.PsiMemorySome_10 >= c.cfg.LowPressureThreshold {
		c.lowPressureSince = time.Time{}
	}

	// High pressure — grow immediately (no cooldown for grow).
	if m.PsiMemorySome_10 >= c.cfg.HighPressureThreshold {
		step := c.cfg.MaxMemoryBytes * uint64(c.cfg.GrowStepPercent) / 100
		if m.PsiMemoryFull_10 > 0 {
			step = step * 3 / 2 // 1.5× step when all tasks are stalled.
		}
		target := min(c.currentBytes+step, c.cfg.MaxMemoryBytes)
		return BalloonAction{
			Type:        BalloonActionGrow,
			TargetBytes: target,
			Reason:      "high memory pressure",
		}
	}

	// Low pressure — consider shrinking.
	if m.PsiMemorySome_10 < c.cfg.LowPressureThreshold {
		// Check cooldown.
		if !c.lastActionTime.IsZero() && time.Since(c.lastActionTime) < c.cfg.Cooldown {
			return none
		}

		// Check idle grace period.
		if time.Since(bootTime) < c.cfg.IdleGracePeriod {
			return none
		}

		// E3: Check settle window — require sustained low pressure before shrinking.
		if c.cfg.SettleWindow > 0 {
			if c.lowPressureSince.IsZero() {
				c.lowPressureSince = time.Now()
				return none // Just started settling.
			}
			if time.Since(c.lowPressureSince) < c.cfg.SettleWindow {
				return none // Still settling.
			}
		}

		// Check swap activity guard.
		if c.cfg.MaxSwapInPerSec > 0 && m.SwapInBytesPerSec > float64(c.cfg.MaxSwapInPerSec) {
			return none
		}

		// E2: Check swap-out activity guard.
		if c.cfg.MaxSwapOutPerSec > 0 && m.SwapOutBytesPerSec > float64(c.cfg.MaxSwapOutPerSec) {
			return none
		}
		// E2: Check page-fault rate guard.
		if c.cfg.MaxPageFaultRate > 0 && m.PageFaultRate > float64(c.cfg.MaxPageFaultRate) {
			return none
		}
		// E2: Check MemAvailable reserve — do not shrink if available memory is too thin.
		if c.cfg.ShrinkReserveBytes > 0 {
			shrinkStep := c.cfg.MaxMemoryBytes * uint64(c.cfg.ShrinkStepPercent) / 100
			if m.MemAvailableBytes < c.cfg.ShrinkReserveBytes+shrinkStep {
				return none
			}
		}

		// Check container activity guards (skip if no containers).
		if m.ContainerCount > 0 {
			if c.cfg.MaxContainerCPU > 0 && m.ContainerCpuPercent > c.cfg.MaxContainerCPU {
				return none
			}
			if c.cfg.MaxContainerIO > 0 && m.ContainerIoBytesPerSec > float64(c.cfg.MaxContainerIO) {
				return none
			}
		}

		// Compute shrink target (multiply before divide to avoid truncation to zero).
		step := c.cfg.MaxMemoryBytes * uint64(c.cfg.ShrinkStepPercent) / 100
		target := c.currentBytes
		if step < target {
			target -= step
		} else {
			target = c.cfg.MinBytes
		}

		// E10-8: Adaptive AnonRss margin based on container activity.
		var marginPct uint64
		switch {
		case m.ContainerCount == 0:
			marginPct = 5 // Idle: kernel + system processes only.
		case m.ContainerCount <= 5:
			marginPct = 15 // Light workload.
		default:
			marginPct = 20 // Heavy workload: more headroom.
		}
		// Apply hard floor: max(min, anon_rss * (1+margin), effectiveFloor).
		floor := c.effectiveFloor()
		hardFloor := max(m.AnonRssBytes+m.AnonRssBytes*marginPct/100, c.cfg.MinBytes, floor)
		if target < hardFloor {
			target = hardFloor
		}

		// Cap at idleTarget if we're above it.
		if target > c.cfg.IdleTargetBytes && c.currentBytes > c.cfg.IdleTargetBytes {
			target = c.cfg.IdleTargetBytes
		} else if c.currentBytes <= c.cfg.IdleTargetBytes {
			return none // Already at or below idle target.
		}

		// Apply hard floor AFTER idleTarget cap so floor always wins.
		if target < hardFloor {
			target = hardFloor
		}

		// Only shrink if target is actually less than current.
		if target >= c.currentBytes {
			return none
		}

		// No-PSI safety: prevent shrink if it would push MemAvailable below the
		// critical grow threshold (15%). Without PSI, this is the only signal we
		// have, and we must avoid shrink→grow oscillation.
		if !c.psiAvailable {
			shrinkAmount := c.currentBytes - target
			if shrinkAmount > m.MemAvailableBytes {
				return none // Shrink would consume more than all available memory.
			}
			expectedAvail := m.MemAvailableBytes - shrinkAmount
			if expectedAvail < target*20/100 {
				return none // Would leave < 20% headroom, risking oscillation.
			}
		}

		// E3: Reset settle window so consecutive shrinks each require a fresh window.
		c.lowPressureSince = time.Time{}

		// E5: After first settle-window shrink, start learning the stable floor.
		if c.effectiveFloor() == 0 && m.ContainerCount == 0 {
			c.TransitionTo(BalloonStateLearningDescend)
		}

		return BalloonAction{
			Type:        BalloonActionShrink,
			TargetBytes: target,
			Reason:      "low pressure shrink",
		}
	}

	return none
}

// evaluateLearningDescend handles the descend phase of floor learning.
func (c *BalloonController) evaluateLearningDescend(m *api.MemoryMetrics, none BalloonAction) BalloonAction {
	// Suspend descend under host pressure Warning/Critical.
	c.mu.Lock()
	hp := c.hostPressure
	c.mu.Unlock()
	if hp != HostPressureNormal {
		return none
	}

	// Detect instability: PSI spike, full stall, or excessive swap-out.
	unstable := m.PsiMemorySome_10 >= c.cfg.HighPressureThreshold || m.PsiMemoryFull_10 > 0 ||
		(c.cfg.MaxSwapOutPerSec > 0 && m.SwapOutBytesPerSec > float64(c.cfg.MaxSwapOutPerSec))
	if unstable {
		step := c.cfg.MaxMemoryBytes * uint64(c.cfg.ShrinkStepPercent) / 100
		c.candidateFloor = min(c.currentBytes+step, c.cfg.IdleTargetBytes)
		c.confirmStartTime = time.Now()
		c.confirmFails = 0
		c.TransitionTo(BalloonStateLearningConfirm)
		return BalloonAction{
			Type: BalloonActionGrow, TargetBytes: c.candidateFloor,
			Reason: "learning: instability detected, confirming floor",
		}
	}

	if time.Since(c.lastActionTime) >= c.cfg.Cooldown {
		step := c.cfg.MaxMemoryBytes * uint64(c.cfg.ShrinkStepPercent) / 100
		if c.currentBytes <= c.cfg.MinBytes+step {
			// Reached min with no instability — set floor just above min.
			c.learnedFloor = c.cfg.MinBytes + step
			c.learnedAt = time.Now()
			c.TransitionTo(BalloonStateSteady)
			_ = store.WriteLearnedFloor(c.instDir, c.learnedFloor, c.learnedAt)
			logrus.Infof("balloon: learned floor at min boundary: %d bytes", c.learnedFloor)
			return none
		}
		target := c.currentBytes - step
		return BalloonAction{
			Type: BalloonActionShrink, TargetBytes: target,
			Reason: "learning: descending to find floor",
		}
	}
	return none
}

// evaluateLearningConfirm handles the confirm phase of floor learning.
func (c *BalloonController) evaluateLearningConfirm(m *api.MemoryMetrics, none BalloonAction) BalloonAction {
	// Detect instability using same signals as descend.
	unstable := m.PsiMemorySome_10 >= c.cfg.HighPressureThreshold || m.PsiMemoryFull_10 > 0 ||
		(c.cfg.MaxSwapOutPerSec > 0 && m.SwapOutBytesPerSec > float64(c.cfg.MaxSwapOutPerSec))
	if unstable {
		c.confirmFails++
		if c.confirmFails >= learnMaxConfirmFails {
			c.learnedFloor = c.cfg.IdleTargetBytes // Conservative fallback.
			c.learnedAt = time.Now()
			c.TransitionTo(BalloonStateSteady)
			_ = store.WriteLearnedFloor(c.instDir, c.learnedFloor, c.learnedAt)
			logrus.Infof("balloon: learning failed %d times, using idleTarget as floor", c.confirmFails)
			return none
		}
		step := c.cfg.MaxMemoryBytes * uint64(c.cfg.ShrinkStepPercent) / 100
		c.candidateFloor = min(c.candidateFloor+step, c.cfg.IdleTargetBytes)
		c.confirmStartTime = time.Now()
		return BalloonAction{
			Type: BalloonActionGrow, TargetBytes: c.candidateFloor,
			Reason: "learning: raising candidate after instability",
		}
	}
	if time.Since(c.confirmStartTime) >= learnConfirmDuration {
		c.learnedFloor = c.candidateFloor
		c.learnedAt = time.Now()
		c.TransitionTo(BalloonStateSteady)
		_ = store.WriteLearnedFloor(c.instDir, c.learnedFloor, c.learnedAt)
		logrus.Infof("balloon: learned stable floor at %d bytes", c.learnedFloor)
		return none
	}
	return none
}
