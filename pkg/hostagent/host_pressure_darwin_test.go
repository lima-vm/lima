// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

//go:build darwin

package hostagent

import (
	"testing"

	"gotest.tools/v3/assert"
)

func TestHostPressure_Hysteresis_NormalToWarning(t *testing.T) {
	m := NewHostPressureMonitor()
	assert.Equal(t, m.Current(), HostPressureNormal)

	// First Warning reading — not enough, stays Normal.
	m.mu.Lock()
	m.transition(HostPressureWarning)
	m.mu.Unlock()
	assert.Equal(t, m.Current(), HostPressureNormal)

	// Second consecutive Warning reading — transitions.
	m.mu.Lock()
	m.transition(HostPressureWarning)
	m.mu.Unlock()
	assert.Equal(t, m.Current(), HostPressureWarning)
}

func TestHostPressure_Hysteresis_WarningToNormal(t *testing.T) {
	m := NewHostPressureMonitor()
	// Get to Warning state first.
	m.mu.Lock()
	m.current = HostPressureWarning
	m.mu.Unlock()

	// First Normal reading — not enough.
	m.mu.Lock()
	m.transition(HostPressureNormal)
	m.mu.Unlock()
	assert.Equal(t, m.Current(), HostPressureWarning)

	// Second consecutive Normal reading — transitions.
	m.mu.Lock()
	m.transition(HostPressureNormal)
	m.mu.Unlock()
	assert.Equal(t, m.Current(), HostPressureNormal)
}

func TestHostPressure_Hysteresis_ImmediateCritical(t *testing.T) {
	m := NewHostPressureMonitor()
	assert.Equal(t, m.Current(), HostPressureNormal)

	// Single Critical reading — immediate transition.
	m.mu.Lock()
	m.transition(HostPressureCritical)
	m.mu.Unlock()
	assert.Equal(t, m.Current(), HostPressureCritical)
}

func TestHostPressure_Hysteresis_CriticalToWarning(t *testing.T) {
	m := NewHostPressureMonitor()
	m.mu.Lock()
	m.current = HostPressureCritical
	m.mu.Unlock()

	// First Warning — not enough.
	m.mu.Lock()
	m.transition(HostPressureWarning)
	m.mu.Unlock()
	assert.Equal(t, m.Current(), HostPressureCritical)

	// Second consecutive Warning — transitions.
	m.mu.Lock()
	m.transition(HostPressureWarning)
	m.mu.Unlock()
	assert.Equal(t, m.Current(), HostPressureWarning)
}

func TestHostPressure_Hysteresis_InterruptResets(t *testing.T) {
	m := NewHostPressureMonitor()

	// One Warning reading.
	m.mu.Lock()
	m.transition(HostPressureWarning)
	m.mu.Unlock()
	assert.Equal(t, m.Current(), HostPressureNormal)

	// Interrupted by Normal — resets pending count.
	m.mu.Lock()
	m.transition(HostPressureNormal)
	m.mu.Unlock()
	assert.Equal(t, m.Current(), HostPressureNormal)

	// One Warning again — counter restarted, not enough.
	m.mu.Lock()
	m.transition(HostPressureWarning)
	m.mu.Unlock()
	assert.Equal(t, m.Current(), HostPressureNormal)
}
