// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package portfwdserver

import (
	"bytes"
	"context"
	"io"
	"net"
	"testing"
	"time"

	"gotest.tools/v3/assert"

	"github.com/lima-vm/lima/v2/pkg/guestagent/api"
)

// recvOnlyTunnelServer implements only Recv of the bidi stream; the other
// methods are never called by GRPCServerRW.Read.
type recvOnlyTunnelServer struct {
	api.GuestService_TunnelServer
	msgs []*api.TunnelMessage
}

func (s *recvOnlyTunnelServer) Recv() (*api.TunnelMessage, error) {
	if len(s.msgs) == 0 {
		return nil, io.EOF
	}
	msg := s.msgs[0]
	s.msgs = s.msgs[1:]
	return msg, nil
}

func TestGRPCServerRWReadShortBuffer(t *testing.T) {
	payload := bytes.Repeat([]byte("x"), 100)
	rw := &GRPCServerRW{stream: &recvOnlyTunnelServer{msgs: []*api.TunnelMessage{{Data: payload}}}}

	var got []byte
	buf := make([]byte, 10)
	for len(got) < len(payload) {
		n, err := rw.Read(buf)
		assert.NilError(t, err)
		assert.Assert(t, n <= len(buf), "Read returned %d, larger than the %d-byte buffer", n, len(buf))
		got = append(got, buf[:n]...)
	}
	assert.DeepEqual(t, got, payload)
}

// TestGRPCServerRWCloseNeverBlocks reproduces the teardown sequence of
// tcpproxy.DialProxy.HandleConn: each copy direction calls CloseRead or
// CloseWrite, and HandleConn itself calls Close, while Start receives from
// closeCh only once. None of the calls may block; a blocked Close used to
// keep HandleConn from closing the dialed guest connection, leaking one FD
// per forwarded connection (https://github.com/lima-vm/lima/issues/5210).
func TestGRPCServerRWCloseNeverBlocks(t *testing.T) {
	rw := &GRPCServerRW{closeCh: make(chan any)}
	done := make(chan struct{})
	go func() {
		_ = rw.CloseWrite()
		_ = rw.CloseRead()
		_ = rw.Close()
		_ = rw.Close() // ctx.Done goroutine in Start may call Close again
		close(done)
	}()
	select {
	case <-rw.closeCh:
	case <-time.After(5 * time.Second):
		assert.Assert(t, false, "closeCh was never signaled")
	}
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		assert.Assert(t, false, "Close/CloseRead/CloseWrite blocked")
	}
}

// fakeTunnelStream is a minimal bidi stream: Recv returns queued messages
// and io.EOF once recvCh is closed; Send discards data.
type fakeTunnelStream struct {
	api.GuestService_TunnelServer
	ctx    context.Context
	recvCh chan *api.TunnelMessage
}

func (s *fakeTunnelStream) Context() context.Context { return s.ctx }

func (s *fakeTunnelStream) Recv() (*api.TunnelMessage, error) {
	msg, ok := <-s.recvCh
	if !ok {
		return nil, io.EOF
	}
	return msg, nil
}

func (s *fakeTunnelStream) Send(*api.TunnelMessage) error { return nil }

// TestTunnelServerClosesGuestConn verifies that the connection dialed to the
// guest service is fully closed (not just shut down for writing) when the
// tunnel stream ends (https://github.com/lima-vm/lima/issues/5210).
func TestTunnelServerClosesGuestConn(t *testing.T) {
	var lc net.ListenConfig
	ln, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	assert.NilError(t, err)
	defer ln.Close()

	acceptCh := make(chan net.Conn, 1)
	go func() {
		conn, err := ln.Accept()
		if err == nil {
			acceptCh <- conn
		}
	}()

	recvCh := make(chan *api.TunnelMessage, 1)
	recvCh <- &api.TunnelMessage{Id: "test", Protocol: "tcp", GuestAddr: ln.Addr().String()}
	stream := &fakeTunnelStream{ctx: t.Context(), recvCh: recvCh}

	startDone := make(chan error, 1)
	go func() {
		startDone <- NewTunnelServer().Start(stream)
	}()

	var guestConn net.Conn
	select {
	case guestConn = <-acceptCh:
	case <-time.After(5 * time.Second):
		assert.Assert(t, false, "tunnel server did not dial the guest address")
	}
	defer guestConn.Close()

	// Simulate the host closing the tunnel.
	close(recvCh)

	select {
	case err := <-startDone:
		assert.NilError(t, err)
	case <-time.After(5 * time.Second):
		assert.Assert(t, false, "Start did not return after the stream ended")
	}

	// If the tunnel server fully closed its side, writes from the guest
	// service eventually fail (RST). With a leaked FD they succeed forever.
	deadline := time.After(5 * time.Second)
	for {
		if _, err := guestConn.Write([]byte("x")); err != nil {
			return
		}
		select {
		case <-deadline:
			assert.Assert(t, false, "guest connection still writable; FD was not closed")
		case <-time.After(10 * time.Millisecond):
		}
	}
}
