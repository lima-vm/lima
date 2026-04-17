// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package store

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// WriteLearnedFloor atomically persists the balloon controller's learned stable floor
// with a timestamp for staleness tracking.
// Format: "<unix_timestamp>\n<floor_bytes>".
func WriteLearnedFloor(instDir string, bytes uint64, learnedAt time.Time) error {
	content := strconv.FormatInt(learnedAt.Unix(), 10) + "\n" + strconv.FormatUint(bytes, 10)
	tmp := filepath.Join(instDir, "learned-floor.tmp")
	if err := os.WriteFile(tmp, []byte(content), 0o600); err != nil {
		return fmt.Errorf("writing learned floor: %w", err)
	}
	return os.Rename(tmp, filepath.Join(instDir, "learned-floor"))
}

// ReadLearnedFloor reads the persisted learned floor and its timestamp.
// Returns (0, zero-time, nil) if not found or corrupt.
// Supports old format (bare uint64) by returning zero time (immediately stale).
func ReadLearnedFloor(instDir string) (uint64, time.Time, error) {
	data, err := os.ReadFile(filepath.Join(instDir, "learned-floor"))
	if errors.Is(err, os.ErrNotExist) {
		return 0, time.Time{}, nil
	}
	if err != nil {
		return 0, time.Time{}, fmt.Errorf("reading learned floor: %w", err)
	}
	content := strings.TrimSpace(string(data))
	lines := strings.SplitN(content, "\n", 2)

	if len(lines) == 2 {
		// New format: "<timestamp>\n<floor>".
		ts, tsErr := strconv.ParseInt(lines[0], 10, 64)
		floor, fErr := strconv.ParseUint(lines[1], 10, 64)
		if tsErr != nil || fErr != nil {
			return 0, time.Time{}, nil // Corrupt.
		}
		return floor, time.Unix(ts, 0), nil
	}

	// Old format: bare uint64 — treat as zero time (immediately stale).
	v, err := strconv.ParseUint(content, 10, 64)
	if err != nil {
		return 0, time.Time{}, nil // Corrupt.
	}
	return v, time.Time{}, nil
}
