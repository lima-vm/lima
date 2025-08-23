// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package hostagent

import (
	"context"
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
	inotifyCache  = make(map[string]int64)
	mountSymlinks = make(map[string]string)
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

			for k, v := range mountSymlinks {
				if strings.HasPrefix(watchPath, k) {
					watchPath = strings.ReplaceAll(watchPath, k, v)
				}
			}
			utcTimestamp := timestamppb.New(stat.ModTime().UTC())
			event := &guestagentapi.Inotify{MountPath: watchPath, Time: utcTimestamp}
			err = inotifyClient.Send(event)
			if err != nil {
				logrus.WithError(err).Warn("failed to send inotify")
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

		logrus.Infof("enable inotify for writable mount: %s", m.Location)
		err = notify.Watch(path.Join(m.Location, "..."), events, GetNotifyEvent())
		if err != nil {
			return err
		}
	}
	return nil
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
