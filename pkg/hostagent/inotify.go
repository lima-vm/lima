// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package hostagent

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/rjeczalik/notify"
	"github.com/sirupsen/logrus"
	"google.golang.org/protobuf/types/known/timestamppb"

	guestagentapi "github.com/lima-vm/lima/v2/pkg/guestagent/api"
)

const CacheSize = 10000

var (
	inotifyCache   = make(map[string]int64)
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
				if !errors.Is(err, os.ErrNotExist) {
					continue
				}
				if guestPath, ok := a.expectRemove(watchPath); ok {
					event := &guestagentapi.Inotify{
						MountPath: guestPath,
						Time:      timestamppb.Now(),
						Removed:   true,
					}
					if err := inotifyClient.Send(event); err != nil {
						return fmt.Errorf("inotify stream closed: %w", err)
					}
				}
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

// expectRemove prepares the relay of the removal of hostPath: the guest agent removes the path in the guest,
// so that the guest emits an inotify event, and the mount must make that removal a no-op on the host,
// as the path may have been created again on the host since.
// It returns the path to remove in the guest, or false when the mount serving hostPath cannot
// make the removal a no-op, e.g., for 9p and virtiofs.
func (a *HostAgent) expectRemove(hostPath string) (string, bool) {
	for symlink, original := range mountSymlinks {
		if isUnder(hostPath, symlink, filepath.Separator) {
			hostPath = original + strings.TrimPrefix(hostPath, symlink)
			break
		}
	}
	// The innermost mount serves the path.
	var served *mount
	for _, m := range a.mounts {
		if isUnder(hostPath, filepath.Clean(m.location), filepath.Separator) &&
			(served == nil || len(filepath.Clean(m.location)) > len(filepath.Clean(served.location))) {
			served = m
		}
	}
	if served == nil || served.expectRemove == nil {
		return "", false
	}
	rel, err := filepath.Rel(filepath.Clean(served.location), hostPath)
	if err != nil {
		return "", false
	}
	guestPath := path.Join(served.mountPoint, filepath.ToSlash(rel))
	// A mount nested in the guest at guestPath would receive the removal, without making it a no-op.
	for _, m := range a.mounts {
		if m != served && isUnder(m.mountPoint, served.mountPoint, '/') && (guestPath == m.mountPoint || isUnder(guestPath, m.mountPoint, '/')) {
			return "", false
		}
	}
	if !served.expectRemove(hostPath) {
		return "", false
	}
	return guestPath, true
}

// isUnder reports whether p is strictly under dir.
func isUnder(p, dir string, sep byte) bool {
	return len(p) > len(dir) && strings.HasPrefix(p, dir) && (p[len(dir)] == sep || strings.HasSuffix(dir, string(sep)))
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
	currTime := stat.ModTime()
	eventPath := event.Path()
	cacheMilli, ok := inotifyCache[eventPath]
	if ok {
		// Ignore repeated events for 10ms to exclude recursive inotify events
		if currTime.UnixMilli()-cacheMilli < 10 {
			return true
		}
	}
	inotifyCache[eventPath] = currTime.UnixMilli()

	if len(inotifyCache) >= CacheSize {
		inotifyCache = make(map[string]int64)
	}
	return false
}
