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
// and io.EOF once recvCh is closed; Send forwards the payload to sendCh, or
// discards it when sendCh is nil.
type fakeTunnelStream struct {
	api.GuestService_TunnelServer
	ctx    context.Context
	recvCh chan *api.TunnelMessage
	sendCh chan []byte
}

func (s *fakeTunnelStream) Context() context.Context { return s.ctx }

func (s *fakeTunnelStream) Recv() (*api.TunnelMessage, error) {
	msg, ok := <-s.recvCh
	if !ok {
		return nil, io.EOF
	}
	return msg, nil
}

func (s *fakeTunnelStream) Send(msg *api.TunnelMessage) error {
	if s.sendCh != nil {
		s.sendCh <- msg.Data
	}
	return nil
}

// TestTunnelServerRelaysHalfClose verifies that a half-close from the host is
// relayed to the guest service as a FIN without tearing the tunnel down, so a
// response written after the half-close still reaches the host
// (https://github.com/lima-vm/lima/issues/5264).
//
// A host-side half-close arrives as Recv returning io.EOF while the stream
// context stays live; only a full close cancels the context as well.
func TestTunnelServerRelaysHalfClose(t *testing.T) {
	var lc net.ListenConfig
	ln, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	assert.NilError(t, err)
	defer ln.Close()

	// The guest service reads the whole request, which completes only once the
	// half-close reaches it, and replies after the test releases replyCh. The
	// handshake stands in for a server that takes time to produce a response.
	reqCh := make(chan []byte, 1)
	errCh := make(chan error, 1)
	replyCh := make(chan struct{})
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			errCh <- err
			return
		}
		defer conn.Close()
		req, err := io.ReadAll(conn)
		if err != nil {
			errCh <- err
			return
		}
		reqCh <- req
		<-replyCh
		_, err = conn.Write([]byte("response"))
		errCh <- err
	}()

	recvCh := make(chan *api.TunnelMessage, 2)
	recvCh <- &api.TunnelMessage{Id: "test", Protocol: "tcp", GuestAddr: ln.Addr().String()}
	recvCh <- &api.TunnelMessage{Id: "test", Data: []byte("request")}
	sendCh := make(chan []byte, 1)
	stream := &fakeTunnelStream{ctx: t.Context(), recvCh: recvCh, sendCh: sendCh}

	startDone := make(chan error, 1)
	go func() {
		startDone <- NewTunnelServer().Start(stream)
	}()

	// Half-close: the host has nothing more to send, but is still waiting for
	// the response.
	close(recvCh)

	select {
	case req := <-reqCh:
		assert.DeepEqual(t, req, []byte("request"))
	case err := <-errCh:
		assert.NilError(t, err)
	case <-time.After(5 * time.Second):
		assert.Assert(t, false, "guest service never saw the half-close")
	}

	close(replyCh)
	assert.NilError(t, <-errCh)

	select {
	case data := <-sendCh:
		assert.DeepEqual(t, data, []byte("response"))
	case <-time.After(5 * time.Second):
		assert.Assert(t, false, "response was lost; the half-close tore the tunnel down")
	}

	select {
	case err := <-startDone:
		assert.NilError(t, err)
	case <-time.After(5 * time.Second):
		assert.Assert(t, false, "Start did not return after the guest service closed")
	}
}

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

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	recvCh := make(chan *api.TunnelMessage, 2)
	recvCh <- &api.TunnelMessage{Id: "test", Protocol: "tcp", GuestAddr: ln.Addr().String()}
	recvCh <- &api.TunnelMessage{Id: "test", Data: []byte("x")}
	stream := &fakeTunnelStream{ctx: ctx, recvCh: recvCh}

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

	// Wait for the tunnel to relay a byte before cancelling. Accept returns as
	// soon as the kernel completes the handshake, which can happen before
	// DialContext returns inside Start, and cancelling in that window makes the
	// dial itself fail instead of exercising the teardown.
	assert.NilError(t, guestConn.SetReadDeadline(time.Now().Add(5*time.Second)))
	buf := make([]byte, 1)
	_, err = io.ReadFull(guestConn, buf)
	assert.NilError(t, err)
	assert.NilError(t, guestConn.SetReadDeadline(time.Time{}))

	// Simulate the host closing the tunnel: GrpcClientRW.Close both stops
	// sending and cancels the stream context. Closing recvCh alone would only
	// be a half-close, which must keep the tunnel up.
	close(recvCh)
	cancel()

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
