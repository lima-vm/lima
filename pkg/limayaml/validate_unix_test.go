//go:build !windows

// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package limayaml

import (
	"testing"

	"gotest.tools/v3/assert"
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
