// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package hostagent

import (
	"context"
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

var (
	inotifyCache   = make(map[string]int64)
	mountSymlinks  = make(map[string]string)
	mountLocations = make(map[string]string)
)

func (a *HostAgent) startInotify(ctx context.Context) error {
	mountWatchCh := make(chan notify.EventInfo, 128)
	err := a.setupWatchers(mountWatchCh)
	if err != nil {
		return err
	}
	client, err := a.getOrCreateClient(ctx)
	if err != nil {
		logrus.WithError(err).Error("failed to create client for inotify")
	}
	inotifyClient, err := client.Inotify(ctx)
	if err != nil {
		return err
	}

	// Trailing-edge debounce: accumulate events and only flush after
	// a quiet period. During bulk operations (yarn install, nuxt build)
	// the timer keeps resetting so no events are forwarded until the
	// burst settles — preventing virtiofs virtqueue contention.
	const quietPeriod = 200 * time.Millisecond
	timer := time.NewTimer(quietPeriod)
	timer.Stop()
	pending := make(map[string]os.FileInfo)

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
			if stat.IsDir() {
				continue
			}
			if filterEvents(watchEvent, stat) {
				continue
			}
			pending[watchPath] = stat
			timer.Reset(quietPeriod)

		case <-timer.C:
			for wp, st := range pending {
				guestPath := translateToGuestPath(wp, mountSymlinks, mountLocations)
				utcTimestamp := timestamppb.New(st.ModTime().UTC())
				event := &guestagentapi.Inotify{MountPath: guestPath, Time: utcTimestamp}
				if err := inotifyClient.Send(event); err != nil {
					logrus.WithError(err).Warn("failed to send inotify")
				}
			}
			pending = make(map[string]os.FileInfo)
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
