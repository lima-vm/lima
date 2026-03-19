// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

//go:build darwin

package store

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"

	"github.com/sirupsen/logrus"
)

// getInstancePhysicalMemory returns the physical memory footprint in bytes
// for a VZ instance by finding the XPC process that has the instance's disk
// file open and querying its memory footprint via the macOS `footprint` command.
func getInstancePhysicalMemory(instDir string) int64 {
	diskPath := filepath.Join(instDir, "disk")
	pid, err := findDiskOwnerPID(diskPath)
	if err != nil {
		logrus.Debugf("Could not find VZ XPC process for %q: %v", instDir, err)
		return 0
	}
	mem, err := getPhysicalFootprint(pid)
	if err != nil {
		logrus.Debugf("Could not get physical footprint for PID %d: %v", pid, err)
		return 0
	}
	return mem
}

// findDiskOwnerPID finds the PID of the process that has the given disk file open
// using the macOS `fuser` command.
func findDiskOwnerPID(diskPath string) (int, error) {
	out, err := exec.CommandContext(context.Background(), "fuser", diskPath).CombinedOutput()
	if err != nil {
		return 0, fmt.Errorf("fuser %q: %w", diskPath, err)
	}
	return parseFuserOutput(string(out))
}

// getPhysicalFootprint returns the physical memory footprint in bytes for a
// given PID using the macOS `footprint` command.
func getPhysicalFootprint(pid int) (int64, error) {
	out, err := exec.CommandContext(context.Background(), "footprint", strconv.Itoa(pid)).Output()
	if err != nil {
		return 0, fmt.Errorf("footprint %d: %w", pid, err)
	}
	return parseFootprintOutput(string(out))
}
