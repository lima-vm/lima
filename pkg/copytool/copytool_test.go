// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package copytool

import (
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
