// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package limatmpl_test

import (
	"testing"

	"gotest.tools/v3/assert"

	"github.com/lima-vm/lima/v2/pkg/limatmpl"
)

func TestTransformCustomURL_GitHub(t *testing.T) {
	testCases := []struct {
		name        string
		input       string
		expected    string
		expectError bool
	}{
		{
			name:     "org only with explicit branch (repo defaults to org)",
			input:    "github:lima-vm@master",
			expected: "https://raw.githubusercontent.com/lima-vm/lima-vm/master/.lima.yaml",
		},
		{
			name:     "org//path with explicit branch (repo defaults to org)",
			input:    "github:lima-vm//templates/docker@master",
			expected: "https://raw.githubusercontent.com/lima-vm/lima-vm/master/templates/docker.yaml",
		},
		{
			name:     "basic org/repo with explicit branch",
			input:    "github:lima-vm/lima@master",
			expected: "https://raw.githubusercontent.com/lima-vm/lima/master/.lima.yaml",
		},
		{
			name:     "org/repo with path and explicit branch",
			input:    "github:lima-vm/lima/templates/docker@master",
			expected: "https://raw.githubusercontent.com/lima-vm/lima/master/templates/docker.yaml",
		},
		{
			name:     "org/repo with path, extension, and explicit branch",
			input:    "github:lima-vm/lima/templates/docker.yaml@master",
			expected: "https://raw.githubusercontent.com/lima-vm/lima/master/templates/docker.yaml",
		},
		{
			name:     "org/repo with trailing slash and explicit branch",
			input:    "github:lima-vm/lima/templates/@main",
			expected: "https://raw.githubusercontent.com/lima-vm/lima/main/templates/lima.yaml",
		},
		{
			name:     "org/repo with tag version",
			input:    "github:lima-vm/lima@v1.0.0",
			expected: "https://raw.githubusercontent.com/lima-vm/lima/v1.0.0/.lima.yaml",
		},
		{
			name:     "org/repo with path and tag",
			input:    "github:lima-vm/lima/templates/alpine@v2.0.0",
			expected: "https://raw.githubusercontent.com/lima-vm/lima/v2.0.0/templates/alpine.yaml",
		},
		{
			name:        "invalid format - empty",
			input:       "github:",
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := limatmpl.TransformCustomURL(t.Context(), tc.input)

			if tc.expectError {
				assert.Assert(t, err != nil, "expected error but got none")
			} else {
				assert.NilError(t, err)
				assert.Equal(t, result, tc.expected)
			}
		})
	}
}

func TestTransformCustomURL_GitHubWithDefaultBranch(t *testing.T) {
	// These tests require network access and will query the GitHub API
	// Skip if running in an environment without network access
	if testing.Short() {
		t.Skip("skipping network-dependent test in short mode")
	}

	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "basic org/repo queries default branch",
			input:    "github:lima-vm/lima",
			expected: "https://raw.githubusercontent.com/lima-vm/lima/master/.lima.yaml",
		},
		{
			name:     "org/repo with path queries default branch",
			input:    "github:lima-vm/lima/templates/docker",
			expected: "https://raw.githubusercontent.com/lima-vm/lima/master/templates/docker.yaml",
		},
		{
			name:     "org with .lima.yaml symlink follows redirect",
			input:    "github:jandubois",
			expected: "https://raw.githubusercontent.com/jandubois/jandubois/main/templates/demo.yaml",
		},
		{
			name:     "org with relative symlink in subdirectory",
			input:    "github:jandubois//docs/lima.yaml",
			expected: "https://raw.githubusercontent.com/jandubois/jandubois/main/templates/demo.yaml",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := limatmpl.TransformCustomURL(t.Context(), tc.input)
			assert.NilError(t, err)
			assert.Equal(t, result, tc.expected)
		})
	}
}

func TestLooksLikeSymlink(t *testing.T) {
	testCases := []struct {
		name     string
		content  string
		expected bool
	}{
		{
			name:     "empty content",
			content:  "",
			expected: false,
		},
		{
			name:     "single line path (not YAML)",
			content:  "templates/docker.yaml",
			expected: true,
		},
		{
			name:     "YAML flow style",
			content:  "{arch: x86_64}",
			expected: false,
		},
		{
			name:     "YAML key-value",
			content:  "arch: x86_64",
			expected: false,
		},
		{
			name:     "multi-line YAML",
			content:  "arch: x86_64\nimages:\n  - location: foo",
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := limatmpl.LooksLikeSymlink(tc.content)
			assert.Equal(t, result, tc.expected)
		})
	}
}
