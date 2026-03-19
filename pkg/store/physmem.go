// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package store

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/docker/go-units"
)

// parseFootprintOutput parses the output of the macOS `footprint` command
// and returns the physical memory footprint in bytes.
func parseFootprintOutput(output string) (int64, error) {
	for line := range strings.SplitSeq(output, "\n") {
		_, after, found := strings.Cut(line, "Footprint:")
		if found {
			// Extract "802 MB" from "Footprint: 802 MB (16384 bytes per page)"
			rest := strings.TrimSpace(after)
			// Remove anything in parentheses.
			if before, _, hasParen := strings.Cut(rest, "("); hasParen {
				rest = strings.TrimSpace(before)
			}
			return parseDecimalSize(rest)
		}
	}
	return 0, errors.New("no Footprint line found in output")
}

// parseDecimalSize parses decimal size strings from macOS `footprint` command output
// (e.g., "802 MB", "1.2 GB"). Returns size in bytes. This is NOT a general-purpose
// size parser — use units.RAMInBytes() for YAML config values.
func parseDecimalSize(s string) (int64, error) {
	parts := strings.Fields(s)
	if len(parts) != 2 {
		return 0, fmt.Errorf("unexpected size format: %q", s)
	}
	val, err := strconv.ParseFloat(parts[0], 64)
	if err != nil {
		return 0, fmt.Errorf("parsing size value %q: %w", parts[0], err)
	}
	switch strings.ToUpper(parts[1]) {
	case "KB":
		return int64(val * 1000), nil
	case "MB":
		return int64(val * 1000 * 1000), nil
	case "GB":
		return int64(val * 1000 * 1000 * 1000), nil
	case "TB":
		return int64(val * 1000 * 1000 * 1000 * 1000), nil
	default:
		return 0, fmt.Errorf("unknown size unit: %q", parts[1])
	}
}

// parseFuserOutput parses the output of the `fuser` command and returns the first PID.
// Expected format: "/path/to/file: 14507" or "/path/to/file: 14507 14508".
func parseFuserOutput(output string) (int, error) {
	output = strings.TrimSpace(output)
	if output == "" {
		return 0, errors.New("empty fuser output")
	}
	_, after, found := strings.Cut(output, ":")
	if !found {
		return 0, fmt.Errorf("no colon separator in fuser output: %q", output)
	}
	fields := strings.Fields(after)
	if len(fields) == 0 {
		return 0, fmt.Errorf("no PID found in fuser output: %q", output)
	}
	pid, err := strconv.Atoi(fields[0])
	if err != nil {
		return 0, fmt.Errorf("parsing PID %q: %w", fields[0], err)
	}
	return pid, nil
}

// FormatMemoryColumn returns a formatted memory string for limactl ls.
// When physical memory is available and significantly less than configured,
// it shows "physical/configured" (e.g., "802MB/8GiB"). Otherwise it shows
// just the configured memory (e.g., "8GiB").
func FormatMemoryColumn(configuredMem, physicalMem int64) string {
	configured := units.BytesSize(float64(configuredMem))
	if physicalMem <= 0 || configuredMem <= 0 {
		return configured
	}
	// Only show physical/configured when physical is significantly less (>10% difference).
	ratio := float64(physicalMem) / float64(configuredMem)
	if ratio >= 0.9 {
		return configured
	}
	physical := units.HumanSize(float64(physicalMem))
	return physical + "/" + configured
}
