// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package ioutilx

import (
	"testing"

	"gotest.tools/v3/assert"
)

func TestWindowsSubsystemPath(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{`C:\Users\varsha\foo`, `/c/Users/varsha/foo`},
		{`D:\projects\lima`, `/d/projects/lima`},
		{`C:\`, `/c/`},
		{`/already/unix/path`, `/already/unix/path`},
	}

	for _, tt := range tests {
		got, err := WindowsSubsystemPath(t.Context(), tt.input)
		assert.NilError(t, err)
		assert.Equal(t, got, tt.expected)
	}
}
