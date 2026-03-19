// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package hostagent

import (
	"time"

	"github.com/sirupsen/logrus"

	"github.com/lima-vm/lima/v2/pkg/guestagent/api"
)

// BalloonState represents the state machine state of the balloon controller.
type BalloonState string

const (
	BalloonStateBootstrap      BalloonState = "bootstrap"
	BalloonStateSteady         BalloonState = "steady"
	BalloonStateAgentFailure   BalloonState = "agent-failure"
	BalloonStateCircuitBreaker BalloonState = "circuit-breaker"
	BalloonStateShutdown       BalloonState = "shutdown"
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
	MaxContainerCPU       float64
	MaxContainerIO        uint64
}

// BalloonController implements the balloon state machine.
// It is NOT safe for concurrent use and must only be called from a single goroutine
// (typically the balloon polling loop in hostagent).
type BalloonController struct {
	cfg          BalloonConfig
	state        BalloonState
	currentBytes uint64

	lastActionTime  time.Time
	pollFailures    int
	oomTimes        []time.Time
	circuitBreakerT time.Time
}

// NewBalloonController creates a controller starting in bootstrap state at max memory.
func NewBalloonController(cfg BalloonConfig) *BalloonController {
	return &BalloonController{
		cfg:          cfg,
		state:        BalloonStateBootstrap,
		currentBytes: cfg.MaxMemoryBytes,
	}
}

// TransitionTo changes the controller state.
func (c *BalloonController) TransitionTo(state BalloonState) {
	logrus.Debugf("balloon: state %s -> %s", c.state, state)
	c.state = state
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

// RecordPollFailure records a failed metrics poll. After 3 failures, expand to max.
func (c *BalloonController) RecordPollFailure() {
	c.pollFailures++
	if c.pollFailures >= 3 {
		c.state = BalloonStateAgentFailure
		c.currentBytes = c.cfg.MaxMemoryBytes
		logrus.Warn("balloon: 3 poll failures, expanding to max memory")
	}
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

	// High pressure — grow immediately (no cooldown for grow).
	if m.PsiMemorySome_10 >= c.cfg.HighPressureThreshold {
		step := c.cfg.MaxMemoryBytes / 100 * uint64(c.cfg.GrowStepPercent)
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

		// Check swap activity guard.
		if c.cfg.MaxSwapInPerSec > 0 && m.SwapInBytesPerSec > float64(c.cfg.MaxSwapInPerSec) {
			return none
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

		// Apply hard floor: max(min, anon_rss * 1.15).
		hardFloor := max(m.AnonRssBytes+m.AnonRssBytes*15/100, c.cfg.MinBytes)
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

		return BalloonAction{
			Type:        BalloonActionShrink,
			TargetBytes: target,
			Reason:      "low pressure shrink",
		}
	}

	return none
}
