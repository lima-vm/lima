// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package metrics

import (
	"testing"

	"gotest.tools/v3/assert"
)

func TestParseCgroupCPUUsage(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    uint64
		wantErr bool
	}{
		{
			name: "typical",
			input: `usage_usec 1234567
user_usec 1000000
system_usec 234567
nr_periods 0
nr_throttled 0
throttled_usec 0
`,
			want: 1234567,
		},
		{
			name:  "zero",
			input: "usage_usec 0\n",
			want:  0,
		},
		{
			name:    "missing field",
			input:   "user_usec 1000000\nsystem_usec 234567\n",
			wantErr: false, // Returns 0, nil when field not found.
			want:    0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseCgroupCPUUsage([]byte(tt.input))
			if tt.wantErr {
				assert.Assert(t, err != nil, "expected error")
				return
			}
			assert.NilError(t, err)
			assert.Equal(t, got, tt.want)
		})
	}
}

func TestParseCgroupIOBytes(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  uint64
	}{
		{
			name:  "single device",
			input: "259:0 rbytes=1048576 wbytes=524288 rios=100 wios=50 dbytes=0 dios=0\n",
			want:  1048576 + 524288,
		},
		{
			name: "multiple devices",
			input: `259:0 rbytes=1000 wbytes=2000 rios=10 wios=20 dbytes=0 dios=0
259:1 rbytes=3000 wbytes=4000 rios=30 wios=40 dbytes=0 dios=0
`,
			want: 1000 + 2000 + 3000 + 4000,
		},
		{
			name:  "empty",
			input: "",
			want:  0,
		},
		{
			name:  "no rbytes or wbytes",
			input: "259:0 rios=100 wios=50\n",
			want:  0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseCgroupIOBytes([]byte(tt.input))
			assert.Equal(t, got, tt.want)
		})
	}
}
