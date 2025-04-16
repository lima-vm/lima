// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package usrlocalsharelima

import (
	"os"
	"path/filepath"
	"testing"

	"gotest.tools/v3/assert"
)

func TestDir(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir := t.TempDir()

	// Create the expected directory structure
	shareDir := filepath.Join(tmpDir, "share", "lima")
	err := os.MkdirAll(shareDir, 0o755)
	assert.NilError(t, err)

	// Create a dummy guest agent binary with the correct name format
	gaBinary := filepath.Join(shareDir, "lima-guestagent.Linux-x86_64")
	err = os.WriteFile(gaBinary, []byte("dummy content"), 0o755)
	assert.NilError(t, err)

	// Create bin directory and limactl file
	binDir := filepath.Join(tmpDir, "bin")
	err = os.MkdirAll(binDir, 0o755)
	assert.NilError(t, err)
	limactlPath := filepath.Join(binDir, "limactl")
	err = os.WriteFile(limactlPath, []byte("dummy content"), 0o755)
	assert.NilError(t, err)

	// Save original value of self
	originalSelf := self
	// Restore original value after test
	defer func() {
		self = originalSelf
	}()
	// Override self for the test
	self = limactlPath

	// Test that Dir() returns the correct path
	dir, err := Dir()
	assert.NilError(t, err)
	assert.Equal(t, dir, shareDir)
}
