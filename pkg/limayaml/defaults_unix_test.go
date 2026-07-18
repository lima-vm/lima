//go:build !windows

// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package limayaml

import (
	"os"
	"path/filepath"
	"testing"

	"gotest.tools/v3/assert"
)

func TestExtractTimezoneFromPath(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test timezone directory structure
	assert.NilError(t, os.MkdirAll(filepath.Join(tmpDir, "Etc"), 0o755))
	assert.NilError(t, os.WriteFile(filepath.Join(tmpDir, "Etc", "UTC"), []byte{}, 0o644))
	assert.NilError(t, os.WriteFile(filepath.Join(tmpDir, "UTC"), []byte{}, 0o644))
	assert.NilError(t, os.MkdirAll(filepath.Join(tmpDir, "Antarctica"), 0o755))
	assert.NilError(t, os.WriteFile(filepath.Join(tmpDir, "Antarctica", "Troll"), []byte{}, 0o644))

	tests := []struct {
		name    string
		path    string
		want    string
		wantErr bool
	}{
		{
			"valid_timezone",
			filepath.Join(tmpDir, "Antarctica", "Troll"),
			"Antarctica/Troll",
			false,
		},
		{
			"root_level_zone",
			filepath.Join(tmpDir, "UTC"),
			"UTC",
			false,
		},
		{
			"outside_zoneinfo",
			"/tmp/somefile",
			"",
			true,
		},
		{
			"empty_path",
			"",
			"",
			true,
		},
		{
			"nonexistent_file",
			filepath.Join(tmpDir, "Invalid", "Zone"),
			"",
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := extractTZFromPath(tt.path)
			if tt.wantErr {
				assert.Assert(t, err != nil, "expected error but got none")
			} else {
				assert.NilError(t, err)
			}
			assert.Equal(t, tt.want, got)
		})
	}
}
