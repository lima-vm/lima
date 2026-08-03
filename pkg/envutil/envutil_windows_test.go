// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package envutil

import (
	"os"
	"path/filepath"
	"testing"

	"gotest.tools/v3/assert"
)

func TestAppendDirsToPath(t *testing.T) {
	sep := string(filepath.ListSeparator)
	existingDir := t.TempDir()
	missingDir := filepath.Join(existingDir, "does-not-exist")
	existingFile := filepath.Join(existingDir, "file.txt")
	assert.NilError(t, os.WriteFile(existingFile, []byte("x"), 0o644))
	dummyCurrentPath := filepath.Join("dummy1", "dummy2")

	testCases := []struct {
		name         string
		originalPATH string
		dirs         []string
		expected     string
	}{
		{
			name:         "existing directory is appended",
			originalPATH: dummyCurrentPath,
			dirs:         []string{existingDir},
			expected:     dummyCurrentPath + sep + existingDir,
		},
		{
			name:         "missing directory is not appended.",
			originalPATH: dummyCurrentPath,
			dirs:         []string{missingDir},
			expected:     dummyCurrentPath,
		},
		{
			name:         "file is not appended",
			originalPATH: dummyCurrentPath,
			dirs:         []string{existingFile},
			expected:     dummyCurrentPath,
		},
		{
			name:         "directory already in pathEnv is not appended again",
			originalPATH: dummyCurrentPath + sep + existingDir,
			dirs:         []string{existingDir},
			expected:     dummyCurrentPath + sep + existingDir,
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			got := AppendDirsToPath(test.originalPATH, test.dirs)
			assert.Equal(t, got, test.expected)
		})
	}
}
