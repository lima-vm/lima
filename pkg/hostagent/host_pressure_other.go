// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !darwin

package hostagent

import "context"

// HostPressureMonitor is a no-op on non-Darwin platforms.
type HostPressureMonitor struct{}

// NewHostPressureMonitor returns a stub monitor that always reports normal pressure.
func NewHostPressureMonitor() *HostPressureMonitor { return &HostPressureMonitor{} }

// Run is a no-op on non-Darwin.
func (m *HostPressureMonitor) Run(_ context.Context) {}

// Current always returns HostPressureNormal on non-Darwin.
func (m *HostPressureMonitor) Current() HostPressure { return HostPressureNormal }
