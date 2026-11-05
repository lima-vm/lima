//go:build !windows

// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package limayaml

import (
	"strings"
	"testing"

	"gotest.tools/v3/assert"

	"github.com/lima-vm/lima/v2/pkg/limatype"
)

func TestValidateMounts(t *testing.T) {
	yBase := `images: [{"location": "/dummy"}]`
	tests := []struct {
		name          string
		mounts        string
		skipOnWindows bool
		wantErr       string
	}{
		{
			name:    "Valid",
			mounts:  `mounts: [{location: "/foo", writable: false}, {location: "~/foo", writable: true}]`,
			wantErr: "",
		},
		{
			name:   "Invalid (relative)",
			mounts: `mounts: [{location: ".", writable: false}]`,
			wantErr: func() string {
				return "must be an absolute path"
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			y, err := Load(t.Context(), []byte(yBase+"\n"+tt.mounts), "lima.yaml")
			assert.NilError(t, err)
			err = Validate(y, false)
			if tt.wantErr != "" {
				assert.ErrorContains(t, err, tt.wantErr)
			} else {
				assert.NilError(t, err)
			}
		})
	}
}

func TestOverlappingMountWarnings(t *testing.T) {
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
			mounts: []limatype.Mount{m("/foo", "/foo"), m("/foobar", "/foobar"), m("/bar", "/mnt/bar")},
		},
		{
			name:     "nested location and mountPoint",
			mounts:   []limatype.Mount{m("/home/user", "/home/user"), m("/home/user/src", "/home/user/src")},
			expected: []string{"`mounts[1].location`", "`mounts[1].mountPoint`"},
		},
		{
			name:     "same location, different mountPoints",
			mounts:   []limatype.Mount{m("/data", "/mnt/a"), m("/data/", "/mnt/b")},
			expected: []string{"`mounts[1].location`"},
		},
		{
			name:     "nested mountPoint only",
			mounts:   []limatype.Mount{m("/a", "/mnt"), m("/b", "/mnt/b")},
			expected: []string{"`mounts[1].mountPoint`"},
		},
		{
			name:     "root location",
			mounts:   []limatype.Mount{m("/", "/host"), m("/data", "/data")},
			expected: []string{"`mounts[1].location`"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			warnings := overlappingMountWarnings(tc.mounts)
			assert.Equal(t, len(warnings), len(tc.expected), "%v", warnings)
			for i, w := range warnings {
				assert.Assert(t, strings.Contains(w, tc.expected[i]), w)
				assert.Assert(t, strings.Contains(w, "overlapping mounts are not supported"), w)
			}
		})
	}
}
