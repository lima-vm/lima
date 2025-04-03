// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package usrlocalsharelima

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// TestDirInTestEnvironment tests that usrlocalsharelima.Dir() works correctly
// when called from tests, where os.Executable is located in a temp directory.
// This test verifies that the guestagent binary can be found relative to the
// test executable's location.
func TestDirInTestEnvironment(t *testing.T) {
	// Get the test executable path (will be in a temp directory)
	testExe, err := os.Executable()
	if err != nil {
		t.Fatalf("Failed to get test executable path: %v", err)
	}

	// Determine architecture for binary name
	arch := "aarch64"
	if runtime.GOARCH == "amd64" {
		arch = "x86_64"
	}
	
	// Create guestagent binary in the first expected location
	// (same directory as test executable)
	gaBinary1 := filepath.Join(filepath.Dir(testExe), fmt.Sprintf("lima-guestagent.Linux-%s", arch))
	err = os.WriteFile(gaBinary1, []byte("dummy"), 0755)
	if err != nil {
		t.Fatalf("Failed to create guestagent binary: %v", err)
	}
	defer os.Remove(gaBinary1)

	// Create guestagent binary in the second expected location
	// (share/lima directory relative to test executable)
	shareDir := filepath.Join(filepath.Dir(filepath.Dir(testExe)), "share", "lima")
	err = os.MkdirAll(shareDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create share directory: %v", err)
	}
	defer os.RemoveAll(shareDir)

	gaBinary2 := filepath.Join(shareDir, fmt.Sprintf("lima-guestagent.Linux-%s", arch))
	err = os.WriteFile(gaBinary2, []byte("dummy"), 0755)
	if err != nil {
		t.Fatalf("Failed to create guestagent binary: %v", err)
	}
	defer os.Remove(gaBinary2)

	// Test that Dir() can find the binary
	dir, err := Dir()
	if err != nil {
		t.Fatalf("Dir() failed: %v", err)
	}

	// Verify that Dir() returns the correct directory
	expectedDir := filepath.Dir(gaBinary1)
	if dir != expectedDir {
		t.Errorf("Expected directory %q, got %q", expectedDir, dir)
	}
} 