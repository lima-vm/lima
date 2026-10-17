//go:build !windows

// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package store

import (
	"sync"
	"sync/atomic"
	"testing"

	"gotest.tools/v3/assert"
)

// TestWriteBootSessionMarkerAtomic has readers race a fixed number of concurrent rewrites of one
// marker with a constant boot ID. An in-place truncating write lets a reader catch the empty file
// and report a previous boot; the atomic rename never does.
func TestWriteBootSessionMarkerAtomic(t *testing.T) {
	dir := t.TempDir()
	const id = "01234567-89ab-cdef-0123-456789abcdef"
	assert.NilError(t, writeBootSessionMarker(dir, id))

	const writesPerWriter = 2000
	var writers, readers sync.WaitGroup
	stop := make(chan struct{})
	var reads, falsePos atomic.Int64

	for range 4 {
		writers.Go(func() {
			for range writesPerWriter {
				if err := writeBootSessionMarker(dir, id); err != nil {
					t.Errorf("writer: %v", err)
					return
				}
			}
		})
	}
	for range 4 {
		readers.Go(func() {
			for {
				select {
				case <-stop:
					return
				default:
				}
				stale, err := staleBootSession(dir, id)
				if err != nil {
					t.Errorf("reader: %v", err)
					return
				}
				reads.Add(1)
				if stale {
					falsePos.Add(1)
				}
			}
		})
	}
	writers.Wait()
	close(stop)
	readers.Wait()

	assert.Assert(t, reads.Load() > 0, "no reads happened")
	assert.Equal(t, falsePos.Load(), int64(0), "staleBootSession reported a previous boot while the boot ID was constant")
}
