// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package hostagent

import (
	"testing"

	"gotest.tools/v3/assert"
)

func TestTranslateToGuestPath(t *testing.T) {
	tests := []struct {
		name      string
		hostPath  string
		symlinks  map[string]string
		locations map[string]string
		expected  string
	}{
		{
			name:      "no translation needed - empty maps",
			hostPath:  "/Users/user/file.txt",
			symlinks:  map[string]string{},
			locations: map[string]string{},
			expected:  "/Users/user/file.txt",
		},
		{
			name:      "no translation needed - location equals mountPoint",
			hostPath:  "/Users/user/project/file.txt",
			symlinks:  map[string]string{},
			locations: map[string]string{"/Users/user": "/Users/user"},
			expected:  "/Users/user/project/file.txt",
		},
		{
			name:      "translate location to different mountPoint",
			hostPath:  "/Users/user/source/file.txt",
			symlinks:  map[string]string{},
			locations: map[string]string{"/Users/user/source": "/mnt/dest"},
			expected:  "/mnt/dest/file.txt",
		},
		{
			name:      "translate location to different mountPoint - nested path",
			hostPath:  "/Users/user/source/subdir/deep/file.txt",
			symlinks:  map[string]string{},
			locations: map[string]string{"/Users/user/source": "/mnt/dest"},
			expected:  "/mnt/dest/subdir/deep/file.txt",
		},
		{
			name:      "translate location to different mountPoint - root file",
			hostPath:  "/Users/user/source/file.txt",
			symlinks:  map[string]string{},
			locations: map[string]string{"/Users/user/source": "/mnt/dest"},
			expected:  "/mnt/dest/file.txt",
		},
		{
			name:      "symlink resolution only",
			hostPath:  "/private/tmp/file.txt",
			symlinks:  map[string]string{"/private/tmp": "/tmp"},
			locations: map[string]string{},
			expected:  "/tmp/file.txt",
		},
		{
			name:      "symlink resolution with location translation",
			hostPath:  "/private/var/folders/source/file.txt",
			symlinks:  map[string]string{"/private/var": "/var"},
			locations: map[string]string{"/var/folders/source": "/mnt/dest"},
			expected:  "/mnt/dest/file.txt",
		},
		{
			name:      "more specific location matches",
			hostPath:  "/Users/user/source/file.txt",
			symlinks:  map[string]string{},
			locations: map[string]string{"/Users/user/source": "/mnt/source"},
			expected:  "/mnt/source/file.txt",
		},
		{
			name:      "less specific location matches when more specific not present",
			hostPath:  "/Users/user/other/file.txt",
			symlinks:  map[string]string{},
			locations: map[string]string{"/Users/user": "/mnt/home"},
			expected:  "/mnt/home/other/file.txt",
		},
		{
			name:      "path not matching any location",
			hostPath:  "/other/path/file.txt",
			symlinks:  map[string]string{},
			locations: map[string]string{"/Users/user/source": "/mnt/dest"},
			expected:  "/other/path/file.txt",
		},
		{
			name:      "exact location match - file at mount root",
			hostPath:  "/Users/user/source",
			symlinks:  map[string]string{},
			locations: map[string]string{"/Users/user/source": "/mnt/dest"},
			expected:  "/mnt/dest",
		},
		{
			name:     "multiple locations - non-overlapping",
			hostPath: "/Users/user/project/file.txt",
			symlinks: map[string]string{},
			locations: map[string]string{
				"/Users/user/project": "/mnt/project",
				"/Users/user/other":   "/mnt/other",
			},
			expected: "/mnt/project/file.txt",
		},
		{
			name:     "multiple symlinks",
			hostPath: "/private/var/tmp/file.txt",
			symlinks: map[string]string{
				"/private/var": "/var",
				"/private/tmp": "/tmp",
			},
			locations: map[string]string{},
			expected:  "/var/tmp/file.txt",
		},
		{
			name:     "multiple locations and symlinks combined",
			hostPath: "/private/var/folders/source/file.txt",
			symlinks: map[string]string{
				"/private/var": "/var",
			},
			locations: map[string]string{
				"/var/folders/source": "/mnt/dest1",
				"/tmp/test":           "/mnt/dest2",
			},
			expected: "/mnt/dest1/file.txt",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := translateToGuestPath(tc.hostPath, tc.symlinks, tc.locations)
			assert.Equal(t, result, tc.expected)
		})
	}
}
