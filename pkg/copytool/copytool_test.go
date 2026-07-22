// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package copytool

import (
	"slices"
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

// TestRsyncCommandEndsOptionParsing verifies that a path starting with a dash is
// passed after "--", so rsync cannot mistake it for an option such as --rsh.
func TestRsyncCommandEndsOptionParsing(t *testing.T) {
	tool, err := newRsyncTool(&Options{})
	if err != nil {
		t.Skip("rsync not found:", err)
	}

	const dashPath = "--rsh=touch pwned"
	// Use local paths to avoid instance lookup
	cmd, err := tool.Command(t.Context(), []string{dashPath, "/tmp/dst"}, nil)
	assert.NilError(t, err)

	sep := slices.Index(cmd.Args, "--")
	assert.Assert(t, sep != -1, "rsync args must contain the %#q separator: %v", "--", cmd.Args)
	assert.Assert(t, slices.Index(cmd.Args, dashPath) > sep, "path %#q must come after %#q: %v", dashPath, "--", cmd.Args)
}
