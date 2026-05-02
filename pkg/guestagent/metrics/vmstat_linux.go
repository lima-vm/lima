// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package metrics

import (
	"bufio"
	"bytes"
	"strconv"
	"strings"
)

// vmstatCounters holds the cumulative counters from /proc/vmstat
// that are relevant for the balloon controller.
type vmstatCounters struct {
	pswpin  uint64 // Pages swapped in (cumulative).
	pswpout uint64 // Pages swapped out (cumulative).
	pgfault uint64 // Page faults (cumulative, major + minor).
	oomKill uint64 // OOM kills (cumulative).
}

// parseProcVmstat extracts swap and OOM counters from /proc/vmstat data.
func parseProcVmstat(data []byte) (vmstatCounters, error) {
	var vs vmstatCounters
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		val, err := strconv.ParseUint(parts[1], 10, 64)
		if err != nil {
			continue
		}
		switch parts[0] {
		case "pswpin":
			vs.pswpin = val
		case "pswpout":
			vs.pswpout = val
		case "pgfault":
			vs.pgfault = val
		case "oom_kill":
			vs.oomKill = val
		}
	}
	return vs, scanner.Err()
}
