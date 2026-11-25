// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package hostagent

import (
	"container/list"
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/rjeczalik/notify"
	"github.com/sirupsen/logrus"
	"google.golang.org/protobuf/types/known/timestamppb"

	guestagentapi "github.com/lima-vm/lima/v2/pkg/guestagent/api"
)

const CacheSize = 10000

// inotifyEchoWindowMs collapses repeated events for one path whose mtimes
// land inside this window. Guest chtimes echoes reuse the forwarded mtime,
// and a host Create followed immediately by Write does too.
const inotifyEchoWindowMs int64 = 10

// inotifyPendingStaleTTL is how long a forwarded path may wait for its echo
// before it can be evicted. A file removed before the guest calls chtimes
// never echoes and must not pin a cache slot forever.
const inotifyPendingStaleTTL = 30 * time.Second

var (
	inotifyCache   = newInotifyEventCache()
	mountSymlinks  = make(map[string]string)
	mountLocations = make(map[string]string)
)

func (a *HostAgent) startInotify(ctx context.Context) error {
	mountWatchCh := make(chan notify.EventInfo, 128)
	if err := a.setupWatchers(mountWatchCh); err != nil {
		return err
	}
	// notify.Watch allocates per-call kernel watchers and an internal reader
	// goroutine; without notify.Stop they leak for the lifetime of the process.
	defer notify.Stop(mountWatchCh)

	// PostInotify returns a usable stream wrapper before the server-side
	// handler is installed, so Sends issued during the gap fail with EOF.
	select {
	case <-a.guestAgentAliveCh:
	case <-ctx.Done():
		return nil
	}

	client, err := a.getOrCreateClient(ctx)
	if err != nil {
		return fmt.Errorf("inotify: failed to obtain guest agent client: %w", err)
	}
	inotifyClient, err := client.Inotify(ctx)
	if err != nil {
		return err
	}
	// Finalize the gRPC client-stream so the guest agent's PostInotify handler
	// can return instead of staying parked on a half-open stream.
	defer func() { _ = inotifyClient.CloseSend() }()

	for {
		select {
		case <-ctx.Done():
			return nil
		case watchEvent := <-mountWatchCh:
			watchPath := watchEvent.Path()
			stat, err := os.Stat(watchPath)
			if err != nil {
				continue
			}

			if filterEvents(watchEvent, stat) {
				continue
			}

			watchPath = translateToGuestPath(watchPath, mountSymlinks, mountLocations)

			utcTimestamp := timestamppb.New(stat.ModTime().UTC())
			event := &guestagentapi.Inotify{MountPath: watchPath, Time: utcTimestamp}
			if err := inotifyClient.Send(event); err != nil {
				// Stream is gone (typically a guest-agent reconnect). Return so
				// the caller can re-spawn against the new client instead of
				// looping silently with a dead stream.
				return fmt.Errorf("inotify stream closed: %w", err)
			}
		}
	}
}

func (a *HostAgent) setupWatchers(events chan notify.EventInfo) error {
	for _, m := range a.instConfig.Mounts {
		if !*m.Writable {
			continue
		}
		symlink, err := filepath.EvalSymlinks(m.Location)
		if err != nil {
			return err
		}
		if m.Location != symlink {
			mountSymlinks[symlink] = m.Location
		}
		if m.MountPoint != nil && m.Location != *m.MountPoint {
			mountLocations[m.Location] = *m.MountPoint
		}

		logrus.Infof("enable inotify for writable mount: %s", m.Location)
		err = notify.Watch(path.Join(m.Location, "..."), events, GetNotifyEvent())
		if err != nil {
			return err
		}
	}
	return nil
}

func translateToGuestPath(hostPath string, symlinks, locations map[string]string) string {
	result := hostPath

	for symlink, original := range symlinks {
		if strings.HasPrefix(result, symlink) {
			result = strings.ReplaceAll(result, symlink, original)
		}
	}

	for location, mountPoint := range locations {
		if suffix, ok := strings.CutPrefix(result, location); ok {
			return mountPoint + suffix
		}
	}

	return result
}

func filterEvents(event notify.EventInfo, stat os.FileInfo) bool {
	return inotifyCache.observe(event.Path(), stat.ModTime())
}

// inotifyCacheEntry is one path remembered so a guest chtimes echo, or a
// second host event inside inotifyEchoWindowMs, is not forwarded again.
type inotifyCacheEntry struct {
	path    string
	modMs   int64
	echoed  bool
	addedAt time.Time
}

// inotifyEventCache is a fixed-capacity cache split between paths whose echo
// is still in flight (pending) and paths that have already suppressed one
// (echoed, ordered LRU with the most recent at the front). Both lists share
// CacheSize. Replacing the map in one shot dropped every in-flight echo at
// once; once enough paths were active those echoes refilled the map and the
// wipe repeated forever.
type inotifyEventCache struct {
	index   map[string]*list.Element
	pending list.List
	echoed  list.List
}

func newInotifyEventCache() *inotifyEventCache {
	return &inotifyEventCache{
		index: make(map[string]*list.Element, CacheSize),
	}
}

func (c *inotifyEventCache) length() int {
	return c.pending.Len() + c.echoed.Len()
}

// observe reports whether this event is an echo that should be dropped.
// A first sighting, or a write whose mtime moved by at least
// inotifyEchoWindowMs, is remembered and forwarded.
func (c *inotifyEventCache) observe(eventPath string, mod time.Time) bool {
	modMs := mod.UnixMilli()
	if elem, ok := c.index[eventPath]; ok {
		entry := elem.Value.(*inotifyCacheEntry)
		if modMs-entry.modMs < inotifyEchoWindowMs {
			c.suppress(elem, entry)
			return true
		}
		entry.modMs = modMs
		entry.addedAt = time.Now()
		c.moveToPendingBack(elem, entry)
		return false
	}
	c.makeRoom()
	entry := &inotifyCacheEntry{
		path:    eventPath,
		modMs:   modMs,
		addedAt: time.Now(),
	}
	c.index[eventPath] = c.pending.PushBack(entry)
	return false
}

// suppress drops a repeated event and records that this path has echoed.
// Echoed entries sit at the front so a later admission evicts the least
// recently echoed path first.
func (c *inotifyEventCache) suppress(elem *list.Element, entry *inotifyCacheEntry) {
	if entry.echoed {
		c.echoed.MoveToFront(elem)
		return
	}
	c.pending.Remove(elem)
	entry.echoed = true
	c.index[entry.path] = c.echoed.PushFront(entry)
}

// moveToPendingBack treats an advanced mtime as a new forwarded write.
// The entry waits at the tail of pending until its echo arrives.
func (c *inotifyEventCache) moveToPendingBack(elem *list.Element, entry *inotifyCacheEntry) {
	if !entry.echoed {
		c.pending.MoveToBack(elem)
		return
	}
	c.echoed.Remove(elem)
	entry.echoed = false
	c.index[entry.path] = c.pending.PushBack(entry)
}

// makeRoom frees one slot for a path the cache has not seen.
// Echoed paths have already done their job, so the LRU echoed entry goes
// first. Otherwise a pending entry older than inotifyPendingStaleTTL goes,
// and only then the newest pending entry. Dropping the newest, rather than
// the oldest, leaves the earlier forwarded prefix in place. A FIFO burst
// larger than CacheSize therefore meets its echoes instead of chasing a
// window that stays permanently ahead of them.
func (c *inotifyEventCache) makeRoom() {
	if c.length() < CacheSize {
		return
	}
	if elem := c.echoed.Back(); elem != nil {
		c.remove(&c.echoed, elem)
		return
	}
	if elem := c.pending.Front(); elem != nil {
		entry := elem.Value.(*inotifyCacheEntry)
		if time.Since(entry.addedAt) >= inotifyPendingStaleTTL {
			c.remove(&c.pending, elem)
			return
		}
	}
	if elem := c.pending.Back(); elem != nil {
		c.remove(&c.pending, elem)
	}
}

func (c *inotifyEventCache) remove(l *list.List, elem *list.Element) {
	entry := elem.Value.(*inotifyCacheEntry)
	l.Remove(elem)
	delete(c.index, entry.path)
}
