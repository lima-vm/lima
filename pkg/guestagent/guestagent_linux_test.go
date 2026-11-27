// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package guestagent

import (
	"context"
	"testing"
	"time"

	"gotest.tools/v3/assert"

	"github.com/lima-vm/lima/v2/pkg/guestagent/api"
	"github.com/lima-vm/lima/v2/pkg/guestagent/ticker"
)

// TestEventsReturnsWhenTheConsumerStopsReading covers the gRPC GetEvents handler
// returning on a stream.Send error while nobody reads the unbuffered channel it
// hands to Events: the send must observe the canceled context instead of blocking
// forever, which would leak the Events goroutine and skip SaveEventState.
func TestEventsReturnsWhenTheConsumerStopsReading(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)

	tt := time.NewTicker(20 * time.Millisecond)
	t.Cleanup(tt.Stop)

	a, err := New(ctx, ticker.NewSimpleTicker(tt), t.TempDir())
	assert.NilError(t, err)

	eventsCtx, eventsCancel := context.WithCancel(t.Context())
	ch := make(chan *api.Event) // unbuffered, as the gRPC handler allocates it
	done := make(chan struct{})
	go func() {
		defer close(done)
		a.Events(eventsCtx, ch)
	}()

	// Nobody reads ch, so Events blocks in its send as soon as it collects an event.
	time.Sleep(200 * time.Millisecond)
	eventsCancel()

	select {
	case <-done:
	case <-time.After(10 * time.Second):
		assert.Assert(t, false, "Events is still blocked on the send after its context was canceled")
	}
}
