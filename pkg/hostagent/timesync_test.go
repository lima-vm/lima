// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package hostagent

import (
	"context"
	"errors"
	"testing"
	"time"

	"gotest.tools/v3/assert"

	guestagentapi "github.com/lima-vm/lima/v2/pkg/guestagent/api"
)

func TestOneWayDelay(t *testing.T) {
	sentAt := time.Date(2026, time.October, 3, 0, 0, 0, 0, time.UTC)

	t.Run("stable link", func(t *testing.T) {
		delay := oneWayDelay(360*time.Millisecond, 360*time.Millisecond)
		assert.Equal(t, delay, 180*time.Millisecond)
		hostTime := compensatedHostTime(sentAt, delay)
		assert.Equal(t, hostTime, sentAt.Add(180*time.Millisecond))
		// Receipt at sentAt+180ms has drift 0, inside the guest's 100ms threshold.
		assert.Equal(t, sentAt.Add(180*time.Millisecond).Sub(hostTime), time.Duration(0))
	})

	t.Run("sustained latency increase", func(t *testing.T) {
		assert.Equal(t, oneWayDelay(80*time.Millisecond, 360*time.Millisecond), 40*time.Millisecond)
		delay := oneWayDelay(360*time.Millisecond, 360*time.Millisecond)
		assert.Equal(t, delay, 180*time.Millisecond)
		hostTime := compensatedHostTime(sentAt, delay)
		receipt := sentAt.Add(180 * time.Millisecond)
		// Guest is still slow by the earlier under-compensation (180ms-40ms).
		guestNow := receipt.Add(-140 * time.Millisecond)
		assert.Equal(t, guestNow.Sub(hostTime), -140*time.Millisecond)
		assert.Equal(t, hostTime, receipt)
	})

	t.Run("single stall", func(t *testing.T) {
		delay := oneWayDelay(200*time.Millisecond, 8000*time.Millisecond)
		assert.Equal(t, delay, 100*time.Millisecond)
		assert.Equal(t, compensatedHostTime(sentAt, delay), sentAt.Add(100*time.Millisecond))
		assert.Equal(t, oneWayDelay(8000*time.Millisecond, 8000*time.Millisecond), 4000*time.Millisecond)
	})

	t.Run("latency drop", func(t *testing.T) {
		assert.Equal(t, oneWayDelay(8000*time.Millisecond, 200*time.Millisecond), 100*time.Millisecond)
	})

	t.Run("startup and empty window", func(t *testing.T) {
		assert.Equal(t, oneWayDelay(0, 0), time.Duration(0))
		assert.Equal(t, compensatedHostTime(sentAt, 0), sentAt)
		assert.Equal(t, oneWayDelay(0, 300*time.Millisecond), 150*time.Millisecond)
	})

	t.Run("non-positive latest sample", func(t *testing.T) {
		assert.Equal(t, oneWayDelay(200*time.Millisecond, 0), time.Duration(0))
		assert.Equal(t, oneWayDelay(200*time.Millisecond, -time.Millisecond), time.Duration(0))
	})
}

func TestSyncGuestClock(t *testing.T) {
	ctx := t.Context()
	sentAt := time.Date(2026, time.October, 3, 0, 0, 0, 0, time.UTC)

	t.Run("startup sends sentAt and records one sample", func(t *testing.T) {
		var got time.Time
		prev, last := syncGuestClock(ctx, func(_ context.Context, hostTime time.Time) (*guestagentapi.TimeSyncResponse, error) {
			got = hostTime
			return &guestagentapi.TimeSyncResponse{}, nil
		}, 0, 0, scriptedNow(sentAt, sentAt.Add(300*time.Millisecond)))
		assert.Equal(t, got, sentAt)
		assert.Equal(t, prev, time.Duration(0))
		assert.Equal(t, last, 300*time.Millisecond)
		assert.Equal(t, oneWayDelay(prev, last), 150*time.Millisecond)
	})

	t.Run("third stable sample keeps the one-way delay", func(t *testing.T) {
		prev, last := 360*time.Millisecond, 360*time.Millisecond
		var got time.Time
		prev, last = syncGuestClock(ctx, func(_ context.Context, hostTime time.Time) (*guestagentapi.TimeSyncResponse, error) {
			got = hostTime
			return &guestagentapi.TimeSyncResponse{}, nil
		}, prev, last, scriptedNow(sentAt, sentAt.Add(360*time.Millisecond)))
		assert.Equal(t, got, compensatedHostTime(sentAt, 180*time.Millisecond))
		assert.Equal(t, prev, 360*time.Millisecond)
		assert.Equal(t, last, 360*time.Millisecond)
		assert.Equal(t, oneWayDelay(prev, last), 180*time.Millisecond)
	})

	t.Run("guest error string still records the sample", func(t *testing.T) {
		prev, last := syncGuestClock(ctx, func(context.Context, time.Time) (*guestagentapi.TimeSyncResponse, error) {
			return &guestagentapi.TimeSyncResponse{Error: "set failed"}, nil
		}, 80*time.Millisecond, 80*time.Millisecond, scriptedNow(sentAt, sentAt.Add(360*time.Millisecond)))
		assert.Equal(t, prev, 80*time.Millisecond)
		assert.Equal(t, last, 360*time.Millisecond)
		assert.Equal(t, oneWayDelay(prev, last), 40*time.Millisecond)
	})

	t.Run("rpc failure returns the previous window", func(t *testing.T) {
		prev, last := syncGuestClock(ctx, func(context.Context, time.Time) (*guestagentapi.TimeSyncResponse, error) {
			return nil, errors.New("rpc failed")
		}, 200*time.Millisecond, 80*time.Millisecond, scriptedNow(sentAt, sentAt.Add(8*time.Second)))
		assert.Equal(t, prev, 200*time.Millisecond)
		assert.Equal(t, last, 80*time.Millisecond)
	})

	t.Run("negative sample is stored as zero", func(t *testing.T) {
		prev, last := syncGuestClock(ctx, func(context.Context, time.Time) (*guestagentapi.TimeSyncResponse, error) {
			return &guestagentapi.TimeSyncResponse{}, nil
		}, 200*time.Millisecond, 80*time.Millisecond, scriptedNow(sentAt, sentAt.Add(-time.Second)))
		assert.Equal(t, prev, 80*time.Millisecond)
		assert.Equal(t, last, time.Duration(0))
		assert.Equal(t, oneWayDelay(prev, last), time.Duration(0))
	})
}

func scriptedNow(times ...time.Time) func() time.Time {
	n := 0
	return func() time.Time {
		t := times[n]
		n++
		return t
	}
}
