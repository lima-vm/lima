// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package metrics

import (
	"bufio"
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// containerCgroupPaths returns cgroup directories for running containers.
// It looks for scope units under well-known systemd slices used by
// Docker, Podman, and containerd (rootful only — cgroupv2).
func containerCgroupPaths() []string {
	var paths []string
	// Docker rootful: /sys/fs/cgroup/system.slice/docker-<id>.scope
	// Podman rootful: /sys/fs/cgroup/machine.slice/libpod-<id>.scope
	// containerd:     /sys/fs/cgroup/system.slice/containerd-<id>.scope (or under containerd.service)
	for _, pattern := range []string{
		"/sys/fs/cgroup/system.slice/docker-*.scope",
		"/sys/fs/cgroup/machine.slice/libpod-*.scope",
		"/sys/fs/cgroup/system.slice/containerd-*.scope",
	} {
		matches, _ := filepath.Glob(pattern)
		paths = append(paths, matches...)
	}
	return paths
}

// cgroupCPUUsage reads cpu.stat from a cgroupv2 directory and returns
// the cumulative usage_usec value (microseconds of CPU time).
func cgroupCPUUsage(cgroupDir string) (uint64, error) {
	data, err := os.ReadFile(filepath.Join(cgroupDir, "cpu.stat"))
	if err != nil {
		return 0, err
	}
	return parseCgroupCPUUsage(data)
}

func parseCgroupCPUUsage(data []byte) (uint64, error) {
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) == 2 && fields[0] == "usage_usec" {
			return strconv.ParseUint(fields[1], 10, 64)
		}
	}
	return 0, scanner.Err()
}

// cgroupIOBytes reads io.stat from a cgroupv2 directory and returns
// the total bytes (read + written) across all devices.
func cgroupIOBytes(cgroupDir string) uint64 {
	data, err := os.ReadFile(filepath.Join(cgroupDir, "io.stat"))
	if err != nil {
		return 0
	}
	return parseCgroupIOBytes(data)
}

// parseCgroupIOBytes parses cgroupv2 io.stat format:
//
//	<major>:<minor> rbytes=<n> wbytes=<n> ...
func parseCgroupIOBytes(data []byte) uint64 {
	var total uint64
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		for _, field := range strings.Fields(scanner.Text()) {
			if after, ok := strings.CutPrefix(field, "rbytes="); ok {
				if v, err := strconv.ParseUint(after, 10, 64); err == nil {
					total += v
				}
			}
			if after, ok := strings.CutPrefix(field, "wbytes="); ok {
				if v, err := strconv.ParseUint(after, 10, 64); err == nil {
					total += v
				}
			}
		}
	}
	return total
}
