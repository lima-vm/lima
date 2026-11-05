// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package limayaml

import (
	"strings"
	"testing"

	"gotest.tools/v3/assert"

	"github.com/lima-vm/lima/v2/pkg/limatype"
)

func TestOverlappingMountWarningsWindows(t *testing.T) {
	m := func(location, mountPoint string) limatype.Mount {
		return limatype.Mount{Location: location, MountPoint: &mountPoint}
	}
	tests := []struct {
		name     string
		mounts   []limatype.Mount
		expected []string
	}{
		{
			name:   "disjoint",
			mounts: []limatype.Mount{m(`C:\data`, "/mnt/a"), m(`C:\database`, "/mnt/b"), m(`D:\data`, "/mnt/c")},
		},
		{
			name:     "drive-letter casing differs",
			mounts:   []limatype.Mount{m(`C:\data`, "/mnt/a"), m(`c:\data\src`, "/mnt/b")},
			expected: []string{"`mounts[1].location`"},
		},
		{
			name:     "path casing differs",
			mounts:   []limatype.Mount{m(`C:\Users\Foo`, "/mnt/a"), m(`C:\users\foo\src`, "/mnt/b")},
			expected: []string{"`mounts[1].location`"},
		},
		{
			name:   "guest mountPoint is case-sensitive",
			mounts: []limatype.Mount{m(`C:\a`, "/mnt/A"), m(`C:\b`, "/mnt/a/b")},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			warnings := overlappingMountWarnings(tc.mounts)
			assert.Equal(t, len(warnings), len(tc.expected), "%v", warnings)
			for i, w := range warnings {
				assert.Assert(t, strings.Contains(w, tc.expected[i]), w)
			}
		})
	}
}
