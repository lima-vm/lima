// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package portfwd

import (
	"context"
	"errors"
	"net"
	"strconv"
	"testing"
	"time"

	"gotest.tools/v3/assert"

	"github.com/lima-vm/lima/v2/pkg/freeport"
)

func hasUDPListener(p *ClosableListeners, k string) bool {
	p.udpListenersRW.Lock()
	defer p.udpListenersRW.Unlock()
	_, ok := p.udpListeners[k]
	return ok
}

func waitUDPListener(p *ClosableListeners, k string) bool {
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if hasUDPListener(p, k) {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return false
}

// A guest can report a port it has already reported, e.g. after the guest agent
// reconnects. The duplicate has to be a no-op, not a teardown of the live listener.
func TestForwardUDPDuplicateKeepsListener(t *testing.T) {
	port, err := freeport.UDP()
	assert.NilError(t, err)
	hostAddress := net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
	guestAddress := "127.0.0.1:60000"
	k := key("udp", hostAddress, guestAddress)

	p := NewClosableListener()
	defer p.Close()

	dialContext := func(_ context.Context, _, _ string) (net.Conn, error) {
		return nil, errors.New("dial is not expected in this test")
	}

	served := make(chan struct{})
	go func() {
		defer close(served)
		p.forwardUDP(t.Context(), dialContext, hostAddress, guestAddress)
	}()
	assert.Assert(t, waitUDPListener(p, k), "timed out waiting for the UDP listener to be registered")

	p.forwardUDP(t.Context(), dialContext, hostAddress, guestAddress)

	assert.Assert(t, hasUDPListener(p, k), "duplicate forward closed the existing listener")
	select {
	case <-served:
		assert.Assert(t, false, "duplicate forward stopped the running listener")
	case <-time.After(100 * time.Millisecond):
	}
}
