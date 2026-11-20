//go:build darwin && !no_vz

// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package vz

import (
	"io"
	"net"
	"testing"
	"time"

	"gotest.tools/v3/assert"
)

// TestVsockProxyClientGoesAway checks that when the TCP client goes away in the
// middle of a session, the guest sees its connection end, and the proxy returns.
func TestVsockProxyClientGoesAway(t *testing.T) {
	// Like vz.VirtioSocketConnection, net.Pipe has no CloseWrite.
	guest, vsock := net.Pipe()
	t.Cleanup(func() { _ = guest.Close() })

	var lc net.ListenConfig
	l, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	assert.NilError(t, err)
	t.Cleanup(func() { _ = l.Close() })
	var d net.Dialer
	client, err := d.DialContext(t.Context(), "tcp", l.Addr().String())
	assert.NilError(t, err)
	t.Cleanup(func() { _ = client.Close() })
	server, err := l.Accept()
	assert.NilError(t, err)

	p := newVsockProxy(func() (net.Conn, error) { return vsock, nil })
	done := make(chan struct{})
	go func() {
		p.HandleConn(server)
		close(done)
	}()

	buf := make([]byte, 4)
	_, err = client.Write([]byte("ping"))
	assert.NilError(t, err)
	_, err = io.ReadFull(guest, buf)
	assert.NilError(t, err)
	assert.Equal(t, string(buf), "ping")
	_, err = guest.Write([]byte("pong"))
	assert.NilError(t, err)
	_, err = io.ReadFull(client, buf)
	assert.NilError(t, err)
	assert.Equal(t, string(buf), "pong")

	// The client goes away while the guest is waiting for input.
	assert.NilError(t, client.Close())

	assert.NilError(t, guest.SetReadDeadline(time.Now().Add(5*time.Second)))
	_, err = guest.Read(buf)
	assert.ErrorIs(t, err, io.EOF)

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		assert.Assert(t, false, "HandleConn did not return after the client went away")
	}
}
