// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

//go:build darwin

package hostagent

import (
	"context"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

// HostPressureMonitor polls macOS kern.memorystatus_level to track host memory pressure.
// Uses hysteresis: transitions require 2 consecutive samples at the new level (except
// transitions TO Critical, which are immediate).
type HostPressureMonitor struct {
	mu             sync.RWMutex
	current        HostPressure
	pending        HostPressure // Candidate state from latest poll.
	pendingCount   int          // Consecutive polls at pending state.
	confirmSamples int          // Required consecutive samples before transition.
}

// NewHostPressureMonitor creates a monitor that defaults to HostPressureNormal.
func NewHostPressureMonitor() *HostPressureMonitor {
	return &HostPressureMonitor{
		confirmSamples: 2, // Require 2 consecutive samples (10s at 5s poll).
	}
}

// Run polls kern.memorystatus_level every 5 seconds until ctx is cancelled.
func (m *HostPressureMonitor) Run(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			m.poll()
		case <-ctx.Done():
			return
		}
	}
}

func (m *HostPressureMonitor) poll() {
	level, err := unix.SysctlUint32("kern.memorystatus_level")
	if err != nil {
		logrus.Debugf("host pressure: kern.memorystatus_level unavailable: %v", err)
		return
	}
	candidate := classifyLevel(level)
	m.mu.Lock()
	defer m.mu.Unlock()
	m.transition(candidate)
}

// transition applies a candidate pressure level with hysteresis.
// Must be called with m.mu held.
func (m *HostPressureMonitor) transition(candidate HostPressure) {
	if candidate == m.current {
		m.pendingCount = 0
		return
	}
	if candidate == m.pending {
		m.pendingCount++
	} else {
		m.pending = candidate
		m.pendingCount = 1
	}
	if candidate == HostPressureCritical || m.pendingCount >= m.confirmSamples {
		m.current = candidate
		m.pendingCount = 0
	}
}

// Current returns the latest host pressure reading.
func (m *HostPressureMonitor) Current() HostPressure {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.current
}
