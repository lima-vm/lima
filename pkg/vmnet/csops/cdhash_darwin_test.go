// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package csops

import (
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"syscall"
	"testing"

	"gotest.tools/v3/assert"
)

// TestMain ensures that the test binary is code-signed before running the tests.
func TestMain(m *testing.M) {
	flag.BoolVar(&signed, "signed", false, "indicates whether the executable is already code-signed")
	flag.Parse()
	if _, filename, _, ok := runtime.Caller(0); !ok {
		log.Fatal("failed to get caller info")
	} else if !signed {
		// declare the script path relative to this source file
		script := filepath.Join(filepath.Dir(filename), "codesign-and-exec.sh")
		// re-exec the current test binary via the codesign-and-exec.sh script
		// with the -signed flag to avoid infinite recursion
		args := append([]string{script, os.Args[0], "-signed"}, os.Args[1:]...)
		if err := syscall.Exec(script, args, os.Environ()); err != nil {
			log.Fatalf("failed to re-exec signed executable: %v", err)
		}
	}
	// run the tests with the signed executable
	m.Run()
}

// signed indicates whether the test executable is code-signed and re-executed via the TestMain function.
var signed bool

// TestCdhashes tests that the Cdhash function correctly detects the code directory hash
// of various processes and compares it with the output of the "codesign" command.
func TestCdhashes(t *testing.T) {
	tests := []struct {
		path string
		pid  int
	}{
		{path: "/sbin/launchd", pid: 1},
		{path: executable(t), pid: os.Getpid()},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("Cdhash(%d)", tt.pid), func(t *testing.T) {
			// Get the expected CDHash via the "codesign" command.
			expected := cdhashViaCodesign(t, tt.path)
			t.Logf("Expected CDHash: %x", expected)

			// Get the CDHash via Cdhash.
			hash, err := Cdhash(tt.pid)
			assert.NilError(t, err, "Cdhash failed for pid %d", tt.pid)
			t.Logf("Cdhash(%d): %x", tt.pid, hash)
			assert.Check(t, slices.Equal(hash, expected), "Cdhash(%d) returned incorrect hash value expected %x, got %x", tt.pid, expected, hash)
		})
	}
}

// executable returns the path to the current executable.
func executable(t *testing.T) string {
	path, err := os.Executable()
	assert.NilError(t, err, "failed to get executable path")
	return path
}

// cdhashViaCodesign retrieves the code directory hash (CDHash) of the given path
// by invoking the "codesign" command.
func cdhashViaCodesign(t *testing.T, path string) []byte {
	display := exec.CommandContext(t.Context(), "codesign", "--display", "-vvv", path)
	output, err := display.CombinedOutput()
	assert.NilError(t, err, "failed to display codesign info for %q: %s\noutput: %s", path, string(output))
	matches := regexp.MustCompile(`(?ms)^\s*CDHash=([0-9a-fA-F]+)`).FindStringSubmatch(string(output))
	assert.Equal(t, len(matches), 2, "failed to parse CDHash from codesign output for %q: %s", path, string(output))
	hash, err := hex.DecodeString(matches[1])
	assert.NilError(t, err, "failed to decode CDHash hex string for %q", path)
	return hash
}
