// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package hostagent

import (
	"context"
	"time"

	"github.com/sirupsen/logrus"

	guestagentapi "github.com/lima-vm/lima/v2/pkg/guestagent/api"
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

	// prev and last are the two most recent successful round trips.
	var prev, last time.Duration
	prev, last = a.syncTimeOnce(ctx, prev, last)

	for {
		select {
		case <-ctx.Done():
			logrus.Debug("Time sync: context cancelled, stopping")
			return
		case <-ticker.C:
			prev, last = a.syncTimeOnce(ctx, prev, last)
		}
	}
}

func (a *HostAgent) syncTimeOnce(ctx context.Context, prev, last time.Duration) (prevRTT, lastRTT time.Duration) {
	client, err := a.getOrCreateClient(ctx)
	if err != nil {
		logrus.WithError(err).Debug("Time sync: failed to get client")
		return prev, last
	}
	return syncGuestClock(ctx, client.SyncTime, prev, last, time.Now)
}

// syncGuestClock sends the predicted guest receipt time and shifts the RTT window
// after SyncTime returns a nil error. An RPC error leaves the window unchanged.
func syncGuestClock(ctx context.Context, doSync func(context.Context, time.Time) (*guestagentapi.TimeSyncResponse, error), prevIn, lastIn time.Duration, now func() time.Time) (prev, last time.Duration) {
	prev, last = prevIn, lastIn
	sentAt := now()
	// host_time is the predicted receipt time (issue 5543).
	// The delay comes from earlier round trips. Capping it at the guest's
	// threshold stops an overestimate from stepping the guest ahead of the host.
	delay := min(oneWayDelay(prev, last), guestagentapi.TimeSyncDriftThreshold)
	hostTime := compensatedHostTime(sentAt, delay)
	resp, err := doSync(ctx, hostTime)
	if err != nil {
		logrus.WithError(err).Debug("Time sync: RPC failed")
		return prev, last
	}

	// A response Error string still counts: the round trip completed.
	prev, last = last, now().Sub(sentAt)

	if resp.Error != "" {
		logrus.Warnf("Time sync: guest failed to set time: %#q (drift was %dms)", resp.Error, resp.DriftMs)
		return prev, last
	}

	if resp.Adjusted {
		logrus.Infof("Time sync: guest clock adjusted (was %dms off)", resp.DriftMs)
	} else {
		logrus.Debugf("Time sync: drift %dms within threshold", resp.DriftMs)
	}
	return prev, last
}

// oneWayDelay estimates host-to-guest delay as half the lesser of the two most
// recent round trips. It is 0 until both samples are positive, so a single
// slow round trip, the first one included, cannot aim the next tick ahead.
func oneWayDelay(prev, last time.Duration) time.Duration {
	if prev <= 0 || last <= 0 {
		return 0
	}
	return min(prev, last) / 2
}

// compensatedHostTime is the predicted guest receipt time sent as host_time.
func compensatedHostTime(sentAt time.Time, delay time.Duration) time.Time {
	return sentAt.Add(delay)
}
