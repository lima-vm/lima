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
	})

	t.Run("sustained latency increase", func(t *testing.T) {
		assert.Equal(t, oneWayDelay(80*time.Millisecond, 360*time.Millisecond), 40*time.Millisecond)
		delay := oneWayDelay(360*time.Millisecond, 360*time.Millisecond)
		assert.Equal(t, delay, 180*time.Millisecond)
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
		assert.Equal(t, oneWayDelay(0, 300*time.Millisecond), time.Duration(0))
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
		assert.Equal(t, oneWayDelay(prev, last), time.Duration(0))
	})

	t.Run("third stable sample caps the one-way delay", func(t *testing.T) {
		prev, last := 360*time.Millisecond, 360*time.Millisecond
		var got time.Time
		prev, last = syncGuestClock(ctx, func(_ context.Context, hostTime time.Time) (*guestagentapi.TimeSyncResponse, error) {
			got = hostTime
			return &guestagentapi.TimeSyncResponse{}, nil
		}, prev, last, scriptedNow(sentAt, sentAt.Add(360*time.Millisecond)))
		assert.Equal(t, got, compensatedHostTime(sentAt, guestagentapi.TimeSyncDriftThreshold))
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
}

func TestSyncGuestClockSteps(t *testing.T) {
	type trip struct{ out, back time.Duration }
	ms := time.Millisecond
	slow, fast, reply := trip{250 * ms, 250 * ms}, trip{5 * ms, 5 * ms}, trip{10 * ms, 790 * ms}
	for _, tc := range []struct {
		name  string
		trips []trip
		ntp   bool
		steps []time.Duration
	}{
		{"steady with NTP", []trip{slow, slow, slow, slow}, true, []time.Duration{-250 * ms, -250 * ms, -150 * ms, -150 * ms}},
		{"first reply stalls", []trip{{10 * ms, 7990 * ms}, fast, fast}, false, nil},
		{"first sync slow", []trip{slow, fast, fast}, false, []time.Duration{-250 * ms, 245 * ms}},
		{"load ends", []trip{fast, fast, slow, slow, slow, fast, fast}, false, []time.Duration{-245 * ms, 340 * ms}},
		{"slow replies end", []trip{fast, fast, reply, reply, reply, fast, fast}, false, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var offset, prev, last time.Duration
			var steps []time.Duration
			for i, tr := range tc.trips {
				if tc.ntp {
					offset = 0
				}
				sentAt := time.Date(2026, time.October, 3, 0, 0, 0, 0, time.UTC).Add(time.Duration(i) * timeSyncInterval)
				prev, last = syncGuestClock(t.Context(), func(_ context.Context, hostTime time.Time) (*guestagentapi.TimeSyncResponse, error) {
					receipt := sentAt.Add(tr.out)
					drift := receipt.Add(offset).Sub(hostTime)
					if drift > 100*ms || drift < -100*ms {
						steps = append(steps, -drift)
						offset = hostTime.Sub(receipt)
					}
					return &guestagentapi.TimeSyncResponse{}, nil
				}, prev, last, scriptedNow(sentAt, sentAt.Add(tr.out+tr.back)))
			}
			assert.DeepEqual(t, steps, tc.steps)
		})
	}
}

func scriptedNow(times ...time.Time) func() time.Time {
	n := 0
	return func() time.Time {
		t := times[n]
		n++
		return t
	}
}
