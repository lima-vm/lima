// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package hostagent

import (
	"path/filepath"
	"testing"

	"gotest.tools/v3/assert"
)

func TestExpectRemove(t *testing.T) {
	// Host paths use the host separator, guest paths always use "/".
	host := filepath.FromSlash
	var got []string
	record := func(name string, supported bool) func(string) bool {
		return func(hostPath string) bool {
			got = append(got, name+":"+hostPath)
			return supported
		}
	}
	a := &HostAgent{mounts: []*mount{
		{location: host("/Users/user/project"), mountPoint: "/mnt/project", expectRemove: record("project", true)},
		{location: host("/Users/user/project/vendor"), mountPoint: "/mnt/project/vendor", expectRemove: record("vendor", true)},
		{location: host("/Users/user/other"), mountPoint: "/mnt/project/sub", expectRemove: record("other", true)},
		{location: host("/Users/user/9p"), mountPoint: "/Users/user/9p", expectRemove: record("9p", false)},
	}}
	mountSymlinks = map[string]string{host("/private/Users/user/project"): host("/Users/user/project")}
	t.Cleanup(func() { mountSymlinks = make(map[string]string) })

	tests := []struct {
		hostPath  string
		wantGuest string
		wantCall  string
	}{
		{hostPath: "/Users/user/project/a.txt", wantGuest: "/mnt/project/a.txt", wantCall: "project:/Users/user/project/a.txt"},
		{hostPath: "/Users/user/project/vendor/b.txt", wantGuest: "/mnt/project/vendor/b.txt", wantCall: "vendor:/Users/user/project/vendor/b.txt"},
		{hostPath: "/private/Users/user/project/c.txt", wantGuest: "/mnt/project/c.txt", wantCall: "project:/Users/user/project/c.txt"},
		{hostPath: "/Users/user/9p/d.txt", wantCall: "9p:/Users/user/9p/d.txt"},
		{hostPath: "/Users/user/project"},
		{hostPath: "/Users/user/project2/e.txt"},
		// Hidden in the guest by the mount of /Users/user/other.
		{hostPath: "/Users/user/project/sub/f.txt"},
		{hostPath: "/Users/user/project/sub"},
		{hostPath: "/Users/user/project/subdir/g.txt", wantGuest: "/mnt/project/subdir/g.txt", wantCall: "project:/Users/user/project/subdir/g.txt"},
	}
	for _, tc := range tests {
		t.Run(tc.hostPath, func(t *testing.T) {
			got = nil
			guestPath, ok := a.expectRemove(host(tc.hostPath))
			assert.Equal(t, guestPath, tc.wantGuest)
			assert.Equal(t, ok, tc.wantGuest != "")
			if tc.wantCall == "" {
				assert.Equal(t, len(got), 0)
			} else {
				assert.DeepEqual(t, got, []string{host(tc.wantCall)})
			}
		})
	}
}

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
