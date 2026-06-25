// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package ioutilx

import (
	"runtime"
	"strings"
	"testing"

	"gotest.tools/v3/assert"
)

// TestWindowsSubsystemPathWithCygpath_Fallback exercises the native
// drive-letter fallback path: when the cygpath binary is unreachable,
// absolute drive-letter inputs convert in-process (e.g. C:\foo -> /c/foo)
// and inputs without a drive letter return an error.
func TestWindowsSubsystemPathWithCygpath_Fallback(t *testing.T) {
	ctx := t.Context()
	// A bogus binary name guarantees the fallback path runs on every
	// platform without inventing a Windows-only test.
	const bogusCygpath = "no-such-cygpath-binary-xyz"

	t.Run("drive-letter backslash", func(t *testing.T) {
		if runtime.GOOS != "windows" {
			t.Skip("filepath.VolumeName / IsAbs treat C:\\foo as a relative path on non-Windows")
		}
		got, err := WindowsSubsystemPathWithCygpath(ctx, bogusCygpath, `C:\Users\USER`)
		assert.NilError(t, err)
		assert.Equal(t, got, "/c/Users/USER")
	})

	t.Run("drive-letter forward-slash", func(t *testing.T) {
		if runtime.GOOS != "windows" {
			t.Skip("filepath.VolumeName / IsAbs only recognize drive letters on Windows")
		}
		got, err := WindowsSubsystemPathWithCygpath(ctx, bogusCygpath, `C:/Users/USER`)
		assert.NilError(t, err)
		assert.Equal(t, got, "/c/Users/USER")
	})

	t.Run("non-drive-letter rejected", func(t *testing.T) {
		// UNC and POSIX inputs both lack a drive letter; on every
		// platform the fallback must refuse rather than fabricate.
		for _, input := range []string{`\\server\share\foo`, "/etc/hosts", "relative.txt"} {
			_, err := WindowsSubsystemPathWithCygpath(ctx, bogusCygpath, input)
			assert.ErrorContains(t, err, "cannot convert", "input=%q", input)
		}
	})

	t.Run("lowercases drive letter", func(t *testing.T) {
		if runtime.GOOS != "windows" {
			t.Skip("Windows-only")
		}
		got, err := WindowsSubsystemPathWithCygpath(ctx, bogusCygpath, `D:\path`)
		assert.NilError(t, err)
		assert.Assert(t, strings.HasPrefix(got, "/d/"), "got %q", got)
	})
}
