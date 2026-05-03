// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package copytool

import (
	"runtime"
	"strings"
	"testing"

	"gotest.tools/v3/assert"
)

// TestCommandDoesNotMutateOptions verifies that passing opts to Command() does not
// overwrite the tool's stored Options for subsequent calls.
func TestCommandDoesNotMutateOptions(t *testing.T) {
	initial := &Options{Verbose: false, Recursive: false}
	tool, err := newRsyncTool(initial)
	if err != nil {
		t.Skip("rsync not found:", err)
	}

	override := &Options{Verbose: true, Recursive: true}
	// Use local paths to avoid instance lookup
	_, _ = tool.Command(t.Context(), []string{"/tmp/src", "/tmp/dst"}, override)

	assert.Equal(t, tool.Options.Verbose, false, "Command() must not mutate stored Options.Verbose")
	assert.Equal(t, tool.Options.Recursive, false, "Command() must not mutate stored Options.Recursive")
}

// TestParseCopyPathsWindowsDriveLetter locks in the distinction between
// Windows absolute paths (local, routed through PathForSSH) and drive-relative
// paths like "C:foo.txt" (not absolute — must be interpreted as instance "C"
// path "foo.txt" so single-letter instance names keep working).
func TestParseCopyPathsWindowsDriveLetter(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows-only path handling")
	}
	ctx := t.Context()

	// Absolute drive-letter paths are local; they must not be classified
	// as instance "C".
	for _, p := range []string{`C:\foo`, `C:/foo`} {
		cps, err := parseCopyPaths(ctx, []string{p})
		if err != nil {
			// A stripped test env may lack ssh; PathForSSH can error.
			// The important guarantee is that the error is not about
			// instance-name lookup.
			assert.Assert(t, !strings.Contains(err.Error(), `instance "C"`),
				"%q must not be classified as instance C: %v", p, err)
			continue
		}
		assert.Equal(t, len(cps), 1)
		assert.Equal(t, cps[0].IsRemote, false, "%q must be a local path", p)
	}

	// "C:foo" is drive-relative (not absolute). It must reach the
	// instance-lookup branch as instance "C" path "foo" and fail at
	// store.Inspect with an instance-not-found error.
	_, err := parseCopyPaths(ctx, []string{"C:foo"})
	assert.ErrorContains(t, err, `instance "C"`)

	// Explicit instance:path behaves the same.
	_, err = parseCopyPaths(ctx, []string{"nonexistent-instance-for-test:/tmp/x"})
	assert.ErrorContains(t, err, `instance "nonexistent-instance-for-test"`)
}
