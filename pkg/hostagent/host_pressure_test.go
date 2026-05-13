// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package hostagent

import (
	"testing"

	"gotest.tools/v3/assert"
)

func TestHostPressureMonitor_ParseLevels(t *testing.T) {
	tests := []struct {
		name  string
		level uint32
		want  HostPressure
	}{
		{"critical low", 0, HostPressureCritical},
		{"critical boundary", 10, HostPressureCritical},
		{"warning boundary low", 11, HostPressureWarning},
		{"warning mid", 20, HostPressureWarning},
		{"warning boundary high", 25, HostPressureWarning},
		{"normal boundary", 26, HostPressureNormal},
		{"normal mid", 50, HostPressureNormal},
		{"normal full", 100, HostPressureNormal},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifyLevel(tt.level)
			assert.Equal(t, got, tt.want)
		})
	}
}
