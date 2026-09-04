// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"testing"
	"testing/synctest"
	"time"

	"gotest.tools/v3/assert"
)

type countingSyncer struct {
	writes, syncedWrites, syncs int
}

func (c *countingSyncer) Write(p []byte) (int, error) { c.writes++; return len(p), nil }
func (c *countingSyncer) Sync() error {
	c.syncedWrites = c.writes
	c.syncs++
	return nil
}

func TestSyncWriter(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c := &countingSyncer{}
		w := &syncWriter{w: c}

		const records = 1000
		for range records {
			_, err := w.Write([]byte("record\n"))
			assert.NilError(t, err)
		}
		time.Sleep(syncInterval)
		synctest.Wait()

		assert.Equal(t, c.writes, records)
		assert.Equal(t, c.syncedWrites, records)
		assert.Equal(t, c.syncs, 1)

		// A coalesced write must be synced even if the stream then goes idle.
		_, err := w.Write([]byte("first\n"))
		assert.NilError(t, err)
		_, err = w.Write([]byte("second\n"))
		assert.NilError(t, err)
		time.Sleep(syncInterval)
		synctest.Wait()

		assert.Equal(t, c.syncedWrites, records+2)
		assert.Equal(t, c.syncs, 2)

		// Shutdown must flush immediately and stop the pending timer.
		_, err = w.Write([]byte("record\n"))
		assert.NilError(t, err)
		w.flush()
		time.Sleep(syncInterval)
		synctest.Wait()

		assert.Equal(t, c.syncedWrites, records+3)
		assert.Equal(t, c.syncs, 3)
	})
}
