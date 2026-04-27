// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package hostagent

import (
	"context"
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
