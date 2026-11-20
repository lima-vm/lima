// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package events

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"gotest.tools/v3/assert"
)

// An event the watcher reads while only part of it has been written must be
// read whole once its newline arrives, not reported as a line of its own.
func TestWatchReadsAHalfWrittenEventWhole(t *testing.T) {
	dir := t.TempDir()
	stdout := filepath.Join(dir, "ha.stdout.log")
	stderr := filepath.Join(dir, "ha.stderr.log")
	for _, path := range []string{stdout, stderr} {
		assert.NilError(t, os.WriteFile(path, nil, 0o644))
	}
	// Each write opens, appends and closes, so that the change is visible to the
	// watcher on every platform (Windows updates an open file's size lazily).
	appendToStdout := func(s string) {
		f, err := os.OpenFile(stdout, os.O_APPEND|os.O_WRONLY, 0o644)
		assert.NilError(t, err)
		_, err = f.WriteString(s)
		assert.NilError(t, err)
		assert.NilError(t, f.Close())
	}

	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	got := make(chan Event, 1)
	done := make(chan error, 1)
	go func() {
		done <- Watch(ctx, stdout, stderr, time.Time{}, false, func(ev Event) bool {
			got <- ev
			return true
		})
	}()

	time.Sleep(500 * time.Millisecond) // let the watcher reach the end of the empty file
	appendToStdout(`{"time":"2026-01-01T00:00:00Z","status":{"running":`)
	time.Sleep(500 * time.Millisecond) // the watcher sees the first half at EOF
	appendToStdout("true}}\n")

	select {
	case ev := <-got:
		assert.Assert(t, ev.Status.Running)
		assert.NilError(t, <-done)
	case err := <-done:
		assert.NilError(t, err, "Watch returned before reporting the event")
		assert.Assert(t, false, "Watch returned without reporting the event")
	}
}
