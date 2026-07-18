// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package limatmpl

import (
	"strings"
	"testing"

	"gotest.tools/v3/assert"
)

func TestSymlinkTarget(t *testing.T) {
	t.Run("target in the same directory", func(t *testing.T) {
		got, err := symlinkTarget("dir/config.yaml", "other.yaml")
		assert.NilError(t, err)
		assert.Equal(t, got, "dir/other.yaml")
	})
	t.Run("target relative to the repo root", func(t *testing.T) {
		got, err := symlinkTarget(".lima.yaml", "sub/x.yaml")
		assert.NilError(t, err)
		assert.Equal(t, got, "sub/x.yaml")
	})
	t.Run("parent that stays within the repo", func(t *testing.T) {
		got, err := symlinkTarget("dir/config.yaml", "../top.yaml")
		assert.NilError(t, err)
		assert.Equal(t, got, "top.yaml")
	})
	t.Run("parent from the repo root is rejected", func(t *testing.T) {
		_, err := symlinkTarget(".lima.yaml", "../evil.yaml")
		assert.ErrorContains(t, err, "escapes the repository")
	})
	t.Run("traversal to another repo is rejected", func(t *testing.T) {
		_, err := symlinkTarget("dir/config.yaml", "../../../../torvalds/linux/master/README")
		assert.ErrorContains(t, err, "escapes the repository")
	})
}

func TestEscapesRepo(t *testing.T) {
	for _, tc := range []struct {
		path string
		want bool
	}{
		{"", false}, // github:ORG has no PATH, and path.Clean("") is "."
		{"x.yaml", false},
		{"dir/../x.yaml", false},
		{"..", true},
		{"../x.yaml", true},
		{"dir/../../x.yaml", true},
	} {
		assert.Equal(t, escapesRepo(tc.path), tc.want, "path %#q", tc.path)
	}
}

// A `..` in REPO or PATH reaches transformGitHubURL from a github: redirect body,
// which validateGitHubRedirect's prefix check alone does not contain.
func TestTransformGitHubURLContainment(t *testing.T) {
	t.Run("path climbing out of the repo is rejected", func(t *testing.T) {
		_, err := transformGitHubURL(t.Context(), "lima-vm/lima/../../../torvalds/linux/master/evil.yaml")
		assert.ErrorContains(t, err, "escapes the repository")
	})
	t.Run("repo of .. is rejected", func(t *testing.T) {
		_, err := transformGitHubURL(t.Context(), "lima-vm/../../torvalds/linux/master/evil.yaml")
		assert.ErrorContains(t, err, "escapes the org")
	})
	t.Run("branch climbing out of the repo is rejected", func(t *testing.T) {
		_, err := transformGitHubURL(t.Context(), "lima-vm/lima/README.md@../../torvalds/linux/master")
		assert.ErrorContains(t, err, "escapes the repository")
	})
	t.Run("redirect body leaving the org is rejected on re-entry", func(t *testing.T) {
		redirect, err := validateGitHubRedirect("github:lima-vm/../../torvalds/linux/master/evil.yaml", "lima-vm", "main", "https://example.com/x")
		assert.NilError(t, err)
		// TransformCustomURL feeds the locator back in as the github: URL's opaque part.
		_, err = transformGitHubURL(t.Context(), strings.TrimPrefix(redirect, "github:"))
		assert.ErrorContains(t, err, "escapes the org")
	})
	t.Run("parent that stays within the repo is allowed", func(t *testing.T) {
		got, err := transformGitHubURL(t.Context(), "lima-vm/lima/docs/../README.md@main")
		assert.NilError(t, err)
		assert.Equal(t, got, "https://raw.githubusercontent.com/lima-vm/lima/main/README.md")
	})
}
