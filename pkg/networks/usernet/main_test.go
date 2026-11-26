// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package usernet

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// exitingDaemon is a root-owned system binary that exits at once.
var exitingDaemon = "/usr/bin/false"

// TestMain points every test at an isolated LIMA_HOME before any of them runs,
// because networks.LoadConfig caches the first networks.yaml it reads for the
// life of the process.
func TestMain(m *testing.M) {
	// Socket paths derived from os.TempDir() exceed UNIX_PATH_MAX on macOS.
	tmp := "/tmp"
	if runtime.GOOS == "windows" {
		tmp = ""
	}
	home, err := os.MkdirTemp(tmp, "lima-usernet-test")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	code := runIn(home, m)
	_ = os.RemoveAll(home)
	os.Exit(code)
}

func runIn(home string, m *testing.M) int {
	if p, err := filepath.EvalSymlinks(exitingDaemon); err == nil {
		exitingDaemon = p
	}
	networksYAML := fmt.Sprintf(`networks:
  user-v2:
    mode: user-v2
    gateway: 192.168.104.1
    netmask: 255.255.255.0
  ext:
    mode: user-v2
    gateway: 192.168.130.1
    netmask: 255.255.255.0
    daemon: %s
`, exitingDaemon)
	cfgDir := filepath.Join(home, "_config")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "networks.yaml"), []byte(networksYAML), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if err := os.Setenv("LIMA_HOME", home); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return m.Run()
}
