// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package store

import (
	"testing"

	"gotest.tools/v3/assert"
)

func TestParseFootprintOutput(t *testing.T) {
	tests := []struct {
		name    string
		output  string
		want    int64
		wantErr bool
	}{
		{
			name: "typical paused VM",
			output: `======================================================================
com.apple.Virtualization.VirtualMachine [14507]: 64-bit    Footprint: 802 MB (16384 bytes per page)
======================================================================`,
			want: 802 * 1000 * 1000,
		},
		{
			name: "running VM with GB",
			output: `======================================================================
com.apple.Virtualization.VirtualMachine [12345]: 64-bit    Footprint: 8 GB (16384 bytes per page)
======================================================================`,
			want: 8 * 1000 * 1000 * 1000,
		},
		{
			name: "small footprint KB",
			output: `======================================================================
com.apple.Virtualization.VirtualMachine [99]: 64-bit    Footprint: 512 KB (16384 bytes per page)
======================================================================`,
			want: 512 * 1000,
		},
		{
			name: "decimal GB footprint",
			output: `======================================================================
com.apple.Virtualization.VirtualMachine [42]: 64-bit    Footprint: 1.2 GB (16384 bytes per page)
======================================================================`,
			want: 1200000000,
		},
		{
			name:    "empty output",
			output:  "",
			wantErr: true,
		},
		{
			name:    "no footprint line",
			output:  "some unrelated output\n",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseFootprintOutput(tt.output)
			if tt.wantErr {
				assert.Assert(t, err != nil, "expected error")
				return
			}
			assert.NilError(t, err)
			assert.Equal(t, got, tt.want)
		})
	}
}

func TestParseFuserOutput(t *testing.T) {
	tests := []struct {
		name    string
		output  string
		want    int
		wantErr bool
	}{
		{
			name:   "typical output",
			output: "/Users/user/.lima/flexd/disk: 14507\n",
			want:   14507,
		},
		{
			name:   "multiple PIDs uses first",
			output: "/Users/user/.lima/flexd/disk: 14507 14508\n",
			want:   14507,
		},
		{
			name:   "spaces around PID",
			output: "/some/path:    99999  \n",
			want:   99999,
		},
		{
			name:    "empty output",
			output:  "",
			wantErr: true,
		},
		{
			name:    "no colon separator",
			output:  "no pid here",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseFuserOutput(tt.output)
			if tt.wantErr {
				assert.Assert(t, err != nil, "expected error")
				return
			}
			assert.NilError(t, err)
			assert.Equal(t, got, tt.want)
		})
	}
}

func TestFormatMemoryColumn(t *testing.T) {
	tests := []struct {
		name           string
		configuredMem  int64
		physicalMem    int64
		expectedOutput string
	}{
		{
			name:           "no physical memory info",
			configuredMem:  8 * 1024 * 1024 * 1024,
			physicalMem:    0,
			expectedOutput: "8GiB",
		},
		{
			name:           "paused VM with reduced memory",
			configuredMem:  8 * 1024 * 1024 * 1024,
			physicalMem:    802 * 1000 * 1000,
			expectedOutput: "802MB/8GiB",
		},
		{
			name:           "running VM at full memory",
			configuredMem:  8 * 1024 * 1024 * 1024,
			physicalMem:    8 * 1024 * 1024 * 1024,
			expectedOutput: "8GiB",
		},
		{
			name:           "physical close to configured (within 10%)",
			configuredMem:  8 * 1024 * 1024 * 1024,
			physicalMem:    int64(7.5 * 1024 * 1024 * 1024),
			expectedOutput: "8GiB",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatMemoryColumn(tt.configuredMem, tt.physicalMem)
			assert.Equal(t, got, tt.expectedOutput)
		})
	}
}
