// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package hostagent

import (
	"container/list"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/rjeczalik/notify"
	"gotest.tools/v3/assert"
)

func TestTranslateToGuestPath(t *testing.T) {
	tests := []struct {
		name      string
		hostPath  string
		symlinks  map[string]string
		locations map[string]string
		expected  string
	}{
		{
			name:      "no translation needed - empty maps",
			hostPath:  "/Users/user/file.txt",
			symlinks:  map[string]string{},
			locations: map[string]string{},
			expected:  "/Users/user/file.txt",
		},
		{
			name:      "no translation needed - location equals mountPoint",
			hostPath:  "/Users/user/project/file.txt",
			symlinks:  map[string]string{},
			locations: map[string]string{"/Users/user": "/Users/user"},
			expected:  "/Users/user/project/file.txt",
		},
		{
			name:      "translate location to different mountPoint",
			hostPath:  "/Users/user/source/file.txt",
			symlinks:  map[string]string{},
			locations: map[string]string{"/Users/user/source": "/mnt/dest"},
			expected:  "/mnt/dest/file.txt",
		},
		{
			name:      "translate location to different mountPoint - nested path",
			hostPath:  "/Users/user/source/subdir/deep/file.txt",
			symlinks:  map[string]string{},
			locations: map[string]string{"/Users/user/source": "/mnt/dest"},
			expected:  "/mnt/dest/subdir/deep/file.txt",
		},
		{
			name:      "translate location to different mountPoint - root file",
			hostPath:  "/Users/user/source/file.txt",
			symlinks:  map[string]string{},
			locations: map[string]string{"/Users/user/source": "/mnt/dest"},
			expected:  "/mnt/dest/file.txt",
		},
		{
			name:      "symlink resolution only",
			hostPath:  "/private/tmp/file.txt",
			symlinks:  map[string]string{"/private/tmp": "/tmp"},
			locations: map[string]string{},
			expected:  "/tmp/file.txt",
		},
		{
			name:      "symlink resolution with location translation",
			hostPath:  "/private/var/folders/source/file.txt",
			symlinks:  map[string]string{"/private/var": "/var"},
			locations: map[string]string{"/var/folders/source": "/mnt/dest"},
			expected:  "/mnt/dest/file.txt",
		},
		{
			name:      "more specific location matches",
			hostPath:  "/Users/user/source/file.txt",
			symlinks:  map[string]string{},
			locations: map[string]string{"/Users/user/source": "/mnt/source"},
			expected:  "/mnt/source/file.txt",
		},
		{
			name:      "less specific location matches when more specific not present",
			hostPath:  "/Users/user/other/file.txt",
			symlinks:  map[string]string{},
			locations: map[string]string{"/Users/user": "/mnt/home"},
			expected:  "/mnt/home/other/file.txt",
		},
		{
			name:      "path not matching any location",
			hostPath:  "/other/path/file.txt",
			symlinks:  map[string]string{},
			locations: map[string]string{"/Users/user/source": "/mnt/dest"},
			expected:  "/other/path/file.txt",
		},
		{
			name:      "exact location match - file at mount root",
			hostPath:  "/Users/user/source",
			symlinks:  map[string]string{},
			locations: map[string]string{"/Users/user/source": "/mnt/dest"},
			expected:  "/mnt/dest",
		},
		{
			name:     "multiple locations - non-overlapping",
			hostPath: "/Users/user/project/file.txt",
			symlinks: map[string]string{},
			locations: map[string]string{
				"/Users/user/project": "/mnt/project",
				"/Users/user/other":   "/mnt/other",
			},
			expected: "/mnt/project/file.txt",
		},
		{
			name:     "multiple symlinks",
			hostPath: "/private/var/tmp/file.txt",
			symlinks: map[string]string{
				"/private/var": "/var",
				"/private/tmp": "/tmp",
			},
			locations: map[string]string{},
			expected:  "/var/tmp/file.txt",
		},
		{
			name:     "multiple locations and symlinks combined",
			hostPath: "/private/var/folders/source/file.txt",
			symlinks: map[string]string{
				"/private/var": "/var",
			},
			locations: map[string]string{
				"/var/folders/source": "/mnt/dest1",
				"/tmp/test":           "/mnt/dest2",
			},
			expected: "/mnt/dest1/file.txt",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := translateToGuestPath(tc.hostPath, tc.symlinks, tc.locations)
			assert.Equal(t, result, tc.expected)
		})
	}
}

type inotifyEvent struct {
	path string
}

func (e inotifyEvent) Event() notify.Event { return notify.Write }
func (e inotifyEvent) Path() string        { return e.path }
func (e inotifyEvent) Sys() any            { return nil }

type inotifyStat struct {
	mod time.Time
}

func (s inotifyStat) Name() string       { return "" }
func (s inotifyStat) Size() int64        { return 0 }
func (s inotifyStat) Mode() os.FileMode  { return 0 }
func (s inotifyStat) ModTime() time.Time { return s.mod }
func (s inotifyStat) IsDir() bool        { return false }
func (s inotifyStat) Sys() any           { return nil }

func sendInotify(path string, mod time.Time) bool {
	return filterEvents(inotifyEvent{path: path}, inotifyStat{mod: mod})
}

func TestFilterEventsDebounceWindow(t *testing.T) {
	inotifyCache = newInotifyEventCache()
	const p = "/mnt/file"
	base := time.Unix(1_700_000_000, 0)

	assert.Equal(t, sendInotify(p, base), false)
	// A chtimes echo reuses the mtime, and so does a second host event inside 10ms.
	assert.Equal(t, sendInotify(p, base), true)
	assert.Equal(t, sendInotify(p, base.Add(9*time.Millisecond)), true)

	later := base.Add(time.Duration(inotifyEchoWindowMs) * time.Millisecond)
	assert.Equal(t, sendInotify(p, later), false)
	assert.Equal(t, sendInotify(p, later), true)
	assert.Equal(t, sendInotify(p, later.Add(9*time.Millisecond)), true)
	assert.Equal(t, sendInotify(p, later.Add(time.Duration(inotifyEchoWindowMs)*time.Millisecond)), false)
}

func TestFilterEventsRefreshesPendingAndEchoed(t *testing.T) {
	inotifyCache = newInotifyEventCache()
	base := time.Unix(1_700_000_000, 0)
	const a, b = "/mnt/a", "/mnt/b"

	assert.Equal(t, sendInotify(a, base), false)
	assert.Equal(t, sendInotify(b, base), false)
	assert.Equal(t, entryPath(inotifyCache.pending.Front()), a)

	// A real write while a is pending forwards again and moves a behind b.
	advanced := base.Add(time.Duration(inotifyEchoWindowMs) * time.Millisecond)
	assert.Equal(t, sendInotify(a, advanced), false)
	assert.Equal(t, entryPath(inotifyCache.pending.Front()), b)
	assert.Equal(t, entryPath(inotifyCache.pending.Back()), a)
	assert.Equal(t, inotifyCache.echoed.Len(), 0)

	// The echo of that write suppresses a and leaves b in flight.
	assert.Equal(t, sendInotify(a, advanced), true)
	assert.Equal(t, inotifyCache.pending.Len(), 1)
	assert.Equal(t, entryPath(inotifyCache.pending.Front()), b)
	assert.Equal(t, inotifyCache.echoed.Len(), 1)
	assert.Equal(t, entryPath(inotifyCache.echoed.Front()), a)

	// Editing a while it is echoed forwards, refreshes the timestamp, and
	// parks the new write at the pending tail until the next echo.
	again := advanced.Add(time.Duration(inotifyEchoWindowMs) * time.Millisecond)
	assert.Equal(t, sendInotify(a, again), false)
	assert.Equal(t, inotifyCache.echoed.Len(), 0)
	assert.Equal(t, entryPath(inotifyCache.pending.Back()), a)
	assert.Equal(t, sendInotify(a, again), true)
	assert.Equal(t, sendInotify(a, again.Add(time.Duration(inotifyEchoWindowMs)*time.Millisecond)), false)
}

func TestFilterEventsCapacityEviction(t *testing.T) {
	base := time.Unix(1_700_000_000, 0)
	inotifyCache = newInotifyEventCache()

	for i := range CacheSize - 1 {
		assert.Equal(t, sendInotify(burstPath(i), base), false)
	}
	assert.Equal(t, inotifyCache.length(), CacheSize-1)
	assert.Equal(t, sendInotify(burstPath(CacheSize-1), base), false)
	assert.Equal(t, inotifyCache.length(), CacheSize)
	_, firstOK := inotifyCache.index[burstPath(0)]
	_, lastOK := inotifyCache.index[burstPath(CacheSize-1)]
	assert.Assert(t, firstOK)
	assert.Assert(t, lastOK)

	// At capacity, with nothing echoed and nothing stale, the newest pending
	// entry is the one that yields. The older prefix stays put.
	assert.Equal(t, sendInotify("/bulk/extra", base), false)
	assert.Equal(t, inotifyCache.length(), CacheSize)
	_, firstOK = inotifyCache.index[burstPath(0)]
	_, evictedOK := inotifyCache.index[burstPath(CacheSize-1)]
	_, extraOK := inotifyCache.index["/bulk/extra"]
	assert.Assert(t, firstOK)
	assert.Assert(t, !evictedOK)
	assert.Assert(t, extraOK)

	inotifyCache = newInotifyEventCache()
	for i := range CacheSize {
		assert.Equal(t, sendInotify(burstPath(i), base), false)
	}
	oldest := inotifyCache.pending.Front().Value.(*inotifyCacheEntry)
	newest := entryPath(inotifyCache.pending.Back())
	oldest.addedAt = time.Now().Add(-inotifyPendingStaleTTL - time.Second)
	assert.Equal(t, sendInotify("/bulk/after-stale", base), false)
	assert.Equal(t, inotifyCache.length(), CacheSize)
	_, oldestOK := inotifyCache.index[oldest.path]
	_, newestOK := inotifyCache.index[newest]
	_, staleExtraOK := inotifyCache.index["/bulk/after-stale"]
	assert.Assert(t, !oldestOK)
	assert.Assert(t, newestOK)
	assert.Assert(t, staleExtraOK)
}

func TestFilterEventsEvictsEchoedBeforeInFlight(t *testing.T) {
	inotifyCache = newInotifyEventCache()
	base := time.Unix(1_700_000_000, 0)

	for i := range CacheSize {
		p := burstPath(i)
		assert.Equal(t, sendInotify(p, base), false)
		assert.Equal(t, sendInotify(p, base), true)
	}
	assert.Equal(t, inotifyCache.length(), CacheSize)
	assert.Equal(t, inotifyCache.pending.Len(), 0)

	const fresh = 100
	for i := range fresh {
		assert.Equal(t, sendInotify(freshPath(i), base), false)
		assert.Equal(t, inotifyCache.length(), CacheSize)
	}
	// The new writes are still waiting on their echoes. The cache dropped
	// the oldest echoed paths to keep those in-flight entries.
	for i := range fresh {
		_, ok := inotifyCache.index[freshPath(i)]
		assert.Assert(t, ok, "in-flight path %d was evicted", i)
		assert.Equal(t, sendInotify(freshPath(i), base), true)
	}
	_, oldestEchoed := inotifyCache.index[burstPath(0)]
	_, recentEchoed := inotifyCache.index[burstPath(CacheSize-1)]
	assert.Assert(t, !oldestEchoed)
	assert.Assert(t, recentEchoed)
	assert.Assert(t, inotifyCache.length() <= CacheSize)
}

func TestFilterEventsFIFOEchoBurstConverges(t *testing.T) {
	const n = 30000
	base := time.Unix(1_700_000_000, 0)
	// The pending prefix that survives MRU eviction is CacheSize-1 entries,
	// so a pure FIFO burst drains in ceil(n/(CacheSize-1)) echo passes.
	// Create+Write duplicates mark a path echoed before the guest echo and
	// spend one extra pass. Wiping the whole map at CacheSize never drains.
	maxPasses := (n + CacheSize - 2) / (CacheSize - 1)

	t.Run("writes then echoes", func(t *testing.T) {
		assertEchoBurstConverges(t, n, base, false, maxPasses)
	})
	t.Run("create and write", func(t *testing.T) {
		assertEchoBurstConverges(t, n, base, true, maxPasses+1)
	})
}

func assertEchoBurstConverges(t *testing.T, n int, base time.Time, duplicate bool, maxPasses int) {
	t.Helper()
	inotifyCache = newInotifyEventCache()
	forwarded := make([]string, 0, n)
	for i := range n {
		p := burstPath(i)
		if !sendInotify(p, base) {
			forwarded = append(forwarded, p)
		}
		if duplicate && !sendInotify(p, base) {
			forwarded = append(forwarded, p)
		}
		assert.Assert(t, inotifyCache.length() <= CacheSize)
	}
	assert.Equal(t, len(forwarded), n)

	echoes := forwarded
	passes := 0
	for len(echoes) > 0 && passes < maxPasses {
		passes++
		next := make([]string, 0, len(echoes))
		for _, p := range echoes {
			if !sendInotify(p, base) {
				next = append(next, p)
			}
			assert.Assert(t, inotifyCache.length() <= CacheSize)
		}
		echoes = next
	}
	assert.Equal(t, len(echoes), 0, "remaining echoes after %d passes (max %d)", passes, maxPasses)
	assert.Assert(t, passes > 1)
	assert.Assert(t, passes <= maxPasses)
}

func burstPath(i int) string {
	return "/bulk/" + strconv.Itoa(i)
}

func freshPath(i int) string {
	return "/fresh/" + strconv.Itoa(i)
}

func entryPath(elem *list.Element) string {
	return elem.Value.(*inotifyCacheEntry).path
}
