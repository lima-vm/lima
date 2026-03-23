// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

// Package metrics reads guest memory statistics from /proc for the balloon controller.
package metrics

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/lima-vm/lima/v2/pkg/guestagent/api"
)

// CollectMemoryMetrics reads /proc/meminfo and /proc/pressure/memory
// and returns a MemoryMetrics protobuf message.
func CollectMemoryMetrics() (*api.MemoryMetrics, error) {
	meminfo, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return nil, fmt.Errorf("failed to read /proc/meminfo: %w", err)
	}
	m, err := parseProcMeminfo(meminfo)
	if err != nil {
		return nil, err
	}

	pressure, _ := os.ReadFile("/proc/pressure/memory")
	psi, err := parseProcPressureMemory(pressure)
	if err != nil {
		return nil, err
	}
	m.PsiMemorySome_10 = psi.Some10
	m.PsiMemoryFull_10 = psi.Full10
	m.PsiMemorySome_60 = psi.Some60
	m.PsiMemoryFull_60 = psi.Full60

	return m, nil
}

func parseProcMeminfo(data []byte) (*api.MemoryMetrics, error) {
	m := &api.MemoryMetrics{}
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		key := strings.TrimSuffix(parts[0], ":")
		val, err := strconv.ParseUint(parts[1], 10, 64)
		if err != nil {
			continue
		}
		// /proc/meminfo values are in kB.
		valBytes := val * 1024
		switch key {
		case "MemTotal":
			m.MemTotalBytes = valBytes
		case "MemAvailable":
			m.MemAvailableBytes = valBytes
		case "Cached":
			m.MemCachedBytes = valBytes
		case "SwapTotal":
			m.SwapTotalBytes = valBytes
		case "SwapFree":
			m.SwapFreeBytes = valBytes
		case "AnonPages":
			m.AnonRssBytes = valBytes
		}
	}
	return m, scanner.Err()
}

// PressureStats holds parsed PSI values from /proc/pressure/memory.
type PressureStats struct {
	Some10, Full10 float64
	Some60, Full60 float64
}

func parseProcPressureMemory(data []byte) (PressureStats, error) {
	var ps PressureStats
	if len(data) == 0 {
		return ps, nil
	}
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		kind := fields[0]
		for _, field := range fields[1:] {
			if after, ok := strings.CutPrefix(field, "avg10="); ok {
				val, parseErr := strconv.ParseFloat(after, 64)
				if parseErr != nil {
					continue
				}
				switch kind {
				case "some":
					ps.Some10 = val
				case "full":
					ps.Full10 = val
				}
			}
			if after, ok := strings.CutPrefix(field, "avg60="); ok {
				val, parseErr := strconv.ParseFloat(after, 64)
				if parseErr != nil {
					continue
				}
				switch kind {
				case "some":
					ps.Some60 = val
				case "full":
					ps.Full60 = val
				}
			}
		}
	}
	return ps, scanner.Err()
}
