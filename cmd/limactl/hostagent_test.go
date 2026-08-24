// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"testing"
	"time"

	"gotest.tools/v3/assert"
)

type countingSyncer struct {
	writes, syncs int
}

func (c *countingSyncer) Write(p []byte) (int, error) { c.writes++; return len(p), nil }
func (c *countingSyncer) Sync() error                 { c.syncs++; return nil }

// TestSyncWriterCoalescesFlushes checks that every record still reaches the
// underlying writer, and that only the flushes are coalesced.
func TestSyncWriterCoalescesFlushes(t *testing.T) {
	c := &countingSyncer{}
	w := &syncWriter{w: c}

	const records = 1000
	for range records {
		_, err := w.Write([]byte("record\n"))
		assert.NilError(t, err)
	}
	assert.Equal(t, c.writes, records)
	assert.Assert(t, c.syncs <= 2, "expected coalesced flushes, got %d for %d records", c.syncs, records)

	// The log must not be able to go unflushed indefinitely.
	before := c.syncs
	time.Sleep(syncInterval + 20*time.Millisecond)
	_, err := w.Write([]byte("record\n"))
	assert.NilError(t, err)
	assert.Equal(t, c.syncs, before+1)
}
