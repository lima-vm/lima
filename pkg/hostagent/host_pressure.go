// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package hostagent

// HostPressure represents the macOS host memory pressure level.
type HostPressure int

const (
	HostPressureNormal HostPressure = iota
	HostPressureWarning
	HostPressureCritical
)

// classifyLevel maps a kern.memorystatus_level value (0-100, higher = more free)
// to a HostPressure level.
func classifyLevel(level uint32) HostPressure {
	switch {
	case level <= 10:
		return HostPressureCritical
	case level <= 25:
		return HostPressureWarning
	default:
		return HostPressureNormal
	}
}
