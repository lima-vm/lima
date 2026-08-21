// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package wsl2

import (
	"testing"

	"gotest.tools/v3/assert"
)

func TestRoutableGuestIP(t *testing.T) {
	tests := []struct {
		name string
		out  string
		want string
	}{
		{"ipv4", "192.168.5.10\n", "192.168.5.10"},
		{"ipv4 trailing space", "10.0.0.5 \n", "10.0.0.5"},
		{"ipv6", "fd00::1\n", "fd00::1"},
		{"loopback rejected", "127.0.1.1\n", ""},
		{"ipv6 loopback rejected", "::1", ""},
		{"empty rejected", "", ""},
		{"ssh option injection rejected", "-oProxyCommand=calc.exe\n", ""},
		{"ssh flag injection rejected", "-J attacker.example:22", ""},
		{"hostname rejected", "attacker.example.com", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, routableGuestIP([]byte(tt.out)), tt.want)
		})
	}
}
