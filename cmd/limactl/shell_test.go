// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"path/filepath"
	"testing"

	"gotest.tools/v3/assert"
)

func TestBuildSyncExcludeArgs(t *testing.T) {
	tests := []struct {
		name      string
		excludes  []string
		hasIgnore bool
		expected  []string
	}{
		{
			name:     "empty excludes, no ignore file",
			excludes: nil,
			expected: nil,
		},
		{
			name:     "single exclude",
			excludes: []string{"node_modules"},
			expected: []string{"--exclude", "node_modules"},
		},
		{
			name:     "multiple excludes",
			excludes: []string{"node_modules", ".git", "vendor"},
			expected: []string{"--exclude", "node_modules", "--exclude", ".git", "--exclude", "vendor"},
		},
		{
			name:      "ignore file only",
			excludes:  nil,
			hasIgnore: true,
			expected:  []string{"--exclude-from", ""}, // placeholder, patched below
		},
		{
			name:      "excludes and ignore file",
			excludes:  []string{".git"},
			hasIgnore: true,
			expected:  []string{"--exclude", ".git", "--exclude-from", ""}, // placeholder
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			if tt.hasIgnore {
				ignoreFile := filepath.Join(dir, ".limasyncignore")
				err := os.WriteFile(ignoreFile, []byte("build\ndist\n"), 0o644)
				assert.NilError(t, err)
				// Patch expected with actual path
				for i, v := range tt.expected {
					if v == "" {
						tt.expected[i] = ignoreFile
					}
				}
			}
			got := buildSyncExcludeArgs(tt.excludes, dir)
			assert.DeepEqual(t, got, tt.expected)
		})
	}
}

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
			name: "metadata-only changes",
			output: `
.d..t...... ./
.f...p..... existing.txt
`,
			expected: &rsyncStats{
				Added:    0,
				Deleted:  0,
				Modified: 0,
				Metadata: 2,
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

func TestValidateSyncFlagValue(t *testing.T) {
	// When the sync value looks like another flag, validation should return an error
	err := validateSyncFlagValue("--sync-exclude=.git")
	assert.ErrorContains(t, err, "--sync flag requires a directory path")

	// When the sync value is a normal directory, validation should pass
	err = validateSyncFlagValue(".")
	assert.NilError(t, err)
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
				Metadata: 4,
			},
			expected: "added: 1, deleted: 2, modified: 3, metadata: 4",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.stats.String(), tt.expected)
		})
	}
}
