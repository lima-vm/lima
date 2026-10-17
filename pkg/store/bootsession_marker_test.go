// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package store

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestWriteBootSessionMarkerAtomic runs concurrent writers and readers of the boot session marker
// with a constant boot ID. Because the ID never changes, staleBootSession must never return true;
// when it does, it read a truncated marker that an in-place write had momentarily emptied. The
// in-place os.WriteFile implementation fails this test and the atomic rename implementation passes.
func TestWriteBootSessionMarkerAtomic(t *testing.T) {
	dir := t.TempDir()
	const id = "01234567-89ab-cdef-0123-456789abcdef"
	if err := writeBootSessionMarker(dir, id); err != nil {
		t.Fatalf("seed marker: %v", err)
	}

	var (
		wg       sync.WaitGroup
		stop     = make(chan struct{})
		reads    atomic.Int64
		falsePos atomic.Int64
	)
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					if err := writeBootSessionMarker(dir, id); err != nil {
						t.Errorf("writer: %v", err)
						return
					}
				}
			}
		}()
	}
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
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
			}
		}()
	}
	time.Sleep(time.Second)
	close(stop)
	wg.Wait()

	if reads.Load() == 0 {
		t.Fatal("no reads happened")
	}
	if n := falsePos.Load(); n != 0 {
		t.Errorf("staleBootSession reported a previous boot %d of %d times while the boot ID was constant", n, reads.Load())
	}
}
