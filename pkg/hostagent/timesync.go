// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package hostagent

import (
	"context"
	"time"

	"github.com/sirupsen/logrus"
)

const (
	timeSyncInterval     = 10 * time.Second
	timeSyncStartupDelay = 5 * time.Second
)

func (a *HostAgent) startTimeSync(ctx context.Context) {
	select {
	case <-a.guestAgentAliveCh:
		logrus.Info("Time sync: guest agent is alive, starting time synchronization")
	case <-ctx.Done():
		return
	}

	select {
	case <-time.After(timeSyncStartupDelay):
	case <-ctx.Done():
		return
	}

	ticker := time.NewTicker(timeSyncInterval)
	defer ticker.Stop()

	a.syncTimeOnce(ctx)

	for {
		select {
		case <-ctx.Done():
			logrus.Debug("Time sync: context cancelled, stopping")
			return
		case <-ticker.C:
			a.syncTimeOnce(ctx)
		}
	}
}

func (a *HostAgent) syncTimeOnce(ctx context.Context) {
	client, err := a.getOrCreateClient(ctx)
	if err != nil {
		logrus.WithError(err).Debug("Time sync: failed to get client")
		return
	}

	hostTime := time.Now()
	resp, err := client.SyncTime(ctx, hostTime)
	if err != nil {
		logrus.WithError(err).Debug("Time sync: RPC failed")
		return
	}

	if resp.Error != "" {
		logrus.Warnf("Time sync: guest failed to set time: %s (drift was %dms)", resp.Error, resp.DriftMs)
		return
	}

	if resp.Adjusted {
		logrus.Infof("Time sync: guest clock adjusted (was %dms off)", resp.DriftMs)
	} else {
		logrus.Debugf("Time sync: drift %dms within threshold", resp.DriftMs)
	}
}
