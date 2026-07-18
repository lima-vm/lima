// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package ioutilx

import (
	"testing"

	"gotest.tools/v3/assert"
)

func TestTranslateWindowsToWSLPath(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    string
		expectedErr string
	}{
		{
			name:     "Standard Windows path",
			input:    `C:\Users\example`,
			expected: "/mnt/c/Users/example",
		},
		{
			name:     "Windows path with forward slashes and trailing slash",
			input:    `C:/Users/example/`,
			expected: "/mnt/c/Users/example",
		},
		{
			name:     "Windows path with trailing backslash",
			input:    `C:\Users\example\`,
			expected: "/mnt/c/Users/example",
		},
		{
			name:     "Drive root only with trailing slash",
			input:    `C:\`,
			expected: "/mnt/c",
		},
		{
			name:     "Drive letter only",
			input:    `c:`,
			expected: "/mnt/c",
		},
		{
			name:        "UNC path starts with double backslash",
			input:       `\\server\share\path`,
			expected:    `\\server\share\path`,
			expectedErr: "UNC paths are not supported for WSL translation",
		},
		{
			name:        "UNC path starts with double slash",
			input:       `//server/share/path`,
			expected:    `//server/share/path`,
			expectedErr: "UNC paths are not supported for WSL translation",
		},
		{
			name:        "Relative path",
			input:       `.\relative\path`,
			expected:    `.\relative\path`,
			expectedErr: "not an absolute Windows path with drive letter",
		},
		{
			name:        "Relative path no dot",
			input:       `relative\path`,
			expected:    `relative\path`,
			expectedErr: "not an absolute Windows path with drive letter",
		},
		{
			name:     "Alternative drive letter",
			input:    `D:\Projects`,
			expected: "/mnt/d/Projects",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual, err := TranslateWindowsToWSLPath(tt.input)
			if tt.expectedErr != "" {
				assert.ErrorContains(t, err, tt.expectedErr)
				assert.Equal(t, actual, tt.expected)
			} else {
				assert.NilError(t, err)
				assert.Equal(t, actual, tt.expected)
			}
		})
	}
}
