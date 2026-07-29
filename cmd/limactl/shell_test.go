// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"testing"

	"gotest.tools/v3/assert"
)

// TestPathDepth covers both host path forms, because `limactl shell --sync`
// compares the depth against rsyncMinimumSrcDirDepth to refuse a directory too
// close to the root. A Windows path must count the same as its Unix equivalent,
// or the same threshold means two different things. The windows column is what
// the call site passes for runtime.GOOS, so both platforms are covered wherever
// this runs.
func TestPathDepth(t *testing.T) {
	tests := []struct {
		path     string
		windows  bool
		expected int
	}{
		// A home directory sits at 3 on either platform, below the minimum of 4.
		{"/Users/jan", false, 3},
		{`C:\Users\jan`, true, 3},
		{"/Users/jan/proj", false, 4},
		{`C:\Users\jan\proj`, true, 4},
		{"/", false, 2},
		{`C:\`, true, 2},
		// filepath.Abs normalises this form away before Lima ever sees it.
		// Counting it anyway keeps the helper independent of the host separator.
		{"C:/Users/jan/proj", true, 4},
		// On Unix a backslash is an ordinary filename character. Counting it as
		// a separator would score this single directory just below the root at
		// 4 and pass the guard.
		{`/x\y\z`, false, 2},
		{`/foo\..\..\..\bar`, false, 2},
		// The guard must refuse a share root and allow a directory inside it;
		// counting both leading separators would pass the root.
		{`\\server\share`, true, 3},
		{`\\server\share\proj`, true, 4},
		// Go's Clean keeps a separator run inside a UNC volume, and whether
		// Windows collapses it earlier is unverified, so the guard collapses
		// the run itself.
		{`\\server\\share`, true, 3},
		{`\\\server\share`, true, 3},
		// An extended-length or device prefix spells a path the plain form
		// also spells.
		{`\\?\C:\`, true, 2},
		{`\\?\C:\Users\jan\proj`, true, 4},
		{`\\.\C:\`, true, 2},
		{`\\?\UNC\server\share`, true, 3},
		// filepath.Clean keeps a root's trailing separator and strips one below
		// a root, so only the first form reaches Lima. Counting both alike
		// keeps the trim from changing an ordinary path's depth.
		{`\\server\share\`, true, 3},
		{`C:\Users\jan\proj\`, true, 4},
		// An empty path returns 1, so the guard refuses one if it ever reaches
		// here.
		{"", false, 1},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			assert.Equal(t, pathDepth(tt.path, tt.windows), tt.expected)
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

func TestWindowsShellCommandFlag(t *testing.T) {
	tests := []struct {
		shell string
		want  string
	}{
		{"cmd.exe", "/c"},
		{"CMD.EXE", "/c"},
		{`C:\Windows\System32\cmd.exe`, "/c"},
		{`C:/Windows/System32/cmd.exe`, "/c"},
		{"powershell.exe", "-Command"},
		{"pwsh.exe", "-Command"},
		{`C:\Program Files\PowerShell\7\pwsh.exe`, "-Command"},
	}
	for _, tt := range tests {
		t.Run(tt.shell, func(t *testing.T) {
			assert.Equal(t, tt.want, windowsShellCommandFlag(tt.shell))
		})
	}
}

func TestWindowsQuoteShell(t *testing.T) {
	tests := []struct {
		shell string
		want  string
	}{
		{"cmd.exe", "cmd.exe"},
		{`C:\Windows\System32\cmd.exe`, `C:\Windows\System32\cmd.exe`},
		{`C:\Program Files\PowerShell\7\pwsh.exe`, `"C:\Program Files\PowerShell\7\pwsh.exe"`},
		{`"C:\already\quoted.exe"`, `"C:\already\quoted.exe"`},
	}
	for _, tt := range tests {
		t.Run(tt.shell, func(t *testing.T) {
			assert.Equal(t, tt.want, windowsQuoteShell(tt.shell))
		})
	}
}
