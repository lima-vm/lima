// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package fsutil

import (
	"strings"
	"testing"

	"gotest.tools/v3/assert"
)

// TestWindowsSubsystemPathWithoutCygpath exercises the native
// drive-letter fallback path: when the cygpath binary is unreachable,
// absolute drive-letter inputs convert in-process (e.g., "C:\foo" -> "/c/foo")
// and inputs without a drive letter return an error.
func TestWindowsSubsystemPathWithoutCygpath(t *testing.T) {
	t.Run("drive-letter backslash", func(t *testing.T) {
		got, err := windowsSubsystemPathWithoutCygpath(`C:\Users\USER`)
		assert.NilError(t, err)
		assert.Equal(t, got, "/c/Users/USER")
	})

	t.Run("drive-letter forward-slash", func(t *testing.T) {
		got, err := windowsSubsystemPathWithoutCygpath(`C:/Users/USER`)
		assert.NilError(t, err)
		assert.Equal(t, got, "/c/Users/USER")
	})

	t.Run("non-drive-letter rejected", func(t *testing.T) {
		// UNC and POSIX inputs both lack a drive letter; on every
		// platform the fallback must refuse rather than fabricate.
		for _, input := range []string{`\\server\share\foo`, "/etc/hosts", "relative.txt"} {
			_, err := windowsSubsystemPathWithoutCygpath(input)
			assert.ErrorContains(t, err, "cannot convert", "input=%q", input)
		}
	})

	t.Run("lowercases drive letter", func(t *testing.T) {
		got, err := windowsSubsystemPathWithoutCygpath(`d:\path`)
		assert.NilError(t, err)
		assert.Assert(t, strings.HasPrefix(got, "/d/"), "got %q", got)
	})
}
