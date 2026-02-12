// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"testing"

	"gotest.tools/v3/assert"
)

func TestParseRsyncStats(t *testing.T) {
	tests := []struct {
		name     string
		output   string
		expected *rsyncStats
	}{
		{
			name: "mixed output",
			output: `
>f+++++++++ new-file.txt
cd+++++++++ dir/
cL+++++++++ new-symlink -> target
*deleting deleted.txt
`,
			expected: &rsyncStats{
				Added:    3,
				Deleted:  1,
				Modified: 0,
			},
		},
		{
			name: "no changes only file list",
			output: `
.d..t...... ./
.f...p..... existing.txt
`,
			expected: &rsyncStats{
				Added:    0,
				Deleted:  0,
				Modified: 0,
			},
		},
		{
			name: "many changes",
			output: `
<f+++++++++ file1
<f+++++++++ file2
*deleting file3
*deleting file4
*deleting file5
<f..T...... file6
`,
			expected: &rsyncStats{
				Added:    2,
				Deleted:  3,
				Modified: 1,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseRsyncStats(tt.output)
			assert.DeepEqual(t, got, tt.expected)
		})
	}
}

func TestRsyncStatsString(t *testing.T) {
	tests := []struct {
		name     string
		stats    *rsyncStats
		expected string
	}{
		{
			name:     "empty",
			stats:    &rsyncStats{},
			expected: "",
		},
		{
			name: "all stats",
			stats: &rsyncStats{
				Added:    1,
				Deleted:  2,
				Modified: 3,
			},
			expected: "added: 1, deleted: 2, modified: 3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.stats.String(), tt.expected)
		})
	}
}
