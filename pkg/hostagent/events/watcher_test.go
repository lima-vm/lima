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

// TestWatchPartialLine writes the first half of an event line before Watch
// starts and the rest later, so the tail reaches EOF mid-line. Watch must
// decode the event only once its newline arrives.
func TestWatchPartialLine(t *testing.T) {
	dir := t.TempDir()
	stdoutPath := filepath.Join(dir, "ha.stdout.log")
	stderrPath := filepath.Join(dir, "ha.stderr.log")
	assert.NilError(t, os.WriteFile(stderrPath, nil, 0o644))

	line := `{"status":{"running":true,"errors":["distinctive-error"]}}` + "\n"
	half := len(line) / 2
	assert.NilError(t, os.WriteFile(stdoutPath, []byte(line[:half]), 0o644))

	written := make(chan struct{})
	go func() {
		defer close(written)
		// Give the tail time to read the fragment. A tail slower than this
		// reads the whole line, so the test cannot tell the fix from the bug.
		time.Sleep(200 * time.Millisecond)
		f, err := os.OpenFile(stdoutPath, os.O_APPEND|os.O_WRONLY, 0)
		if err != nil {
			t.Error(err)
			return
		}
		defer f.Close()
		if _, err := f.WriteString(line[half:]); err != nil {
			t.Error(err)
		}
	}()

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	var received *Event
	err := Watch(ctx, stdoutPath, stderrPath, time.Time{}, false, func(ev Event) bool {
		received = &ev
		return true
	})
	<-written
	assert.NilError(t, err)
	assert.Assert(t, received != nil, "Watch returned without an event: %v", ctx.Err())
	assert.Assert(t, received.Status.Running)
	assert.DeepEqual(t, received.Status.Errors, []string{"distinctive-error"})
}
