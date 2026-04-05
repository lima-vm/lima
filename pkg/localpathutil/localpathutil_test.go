// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package localpathutil

import (
	"os"
	"path/filepath"
	"testing"

	"gotest.tools/v3/assert"
)

func TestExpand(t *testing.T) {
	homeDir, err := os.UserHomeDir()
	assert.NilError(t, err)

	absDir := t.TempDir()

	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			name:  "tilde only",
			input: "~",
			want:  homeDir,
		},
		{
			name:  "tilde with subpath",
			input: "~/foo/bar",
			want:  filepath.Join(homeDir, "foo/bar"),
		},
		{
			name:  "absolute path",
			input: absDir,
			want:  absDir,
		},
		{
			name:    "empty path",
			input:   "",
			wantErr: true,
		},
		{
			name:    "unsupported tilde user path",
			input:   "~foo/bar",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Expand(tt.input)
			if tt.wantErr {
				assert.Assert(t, err != nil, "expected error for input %q", tt.input)
				return
			}
			assert.NilError(t, err)
			assert.Equal(t, got, tt.want)
		})
	}
}
