// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package portfwd

import (
	"io"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"gotest.tools/v3/assert"

	"github.com/lima-vm/lima/v2/pkg/guestagent/api"
	guestagentclient "github.com/lima-vm/lima/v2/pkg/guestagent/api/client"
	"github.com/lima-vm/lima/v2/pkg/portfwdserver"
)

type tunnelGuestService struct {
	api.UnimplementedGuestServiceServer
	tunnel *portfwdserver.TunnelServer
}

func (s *tunnelGuestService) Tunnel(stream api.GuestService_TunnelServer) error {
	return s.tunnel.Start(stream)
}

func shortUnixTempDir(t *testing.T) string {
	t.Helper()
	//nolint:usetesting // t.TempDir() may return a path too long for a Unix socket.
	dir, err := os.MkdirTemp("", "lima")
	assert.NilError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return dir
}

func echoUnixListener(t *testing.T, sockPath string) net.Listener {
	t.Helper()
	var listenConfig net.ListenConfig
	ln, err := listenConfig.Listen(t.Context(), "unix", sockPath)
	assert.NilError(t, err)
	t.Cleanup(func() { ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func() {
				defer conn.Close()
				_, _ = io.Copy(conn, conn)
			}()
		}
	}()
	return ln
}

func dialWithRetry(t *testing.T, hostSocket string) net.Conn {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		var dialer net.Dialer
		conn, err := dialer.DialContext(t.Context(), "unix", hostSocket)
		if err == nil {
			return conn
		}
		lastErr = err
		time.Sleep(20 * time.Millisecond)
	}
	assert.NilError(t, lastErr, "could not dial %s", hostSocket)
	return nil
}

func TestForwardUnixProxiesData(t *testing.T) {
	dir := shortUnixTempDir(t)
	guestSocket := filepath.Join(dir, "guest.sock")
	hostSocket := filepath.Join(dir, "host.sock")

	echoUnixListener(t, guestSocket)

	client := newTestGuestAgentClient(t, &tunnelGuestService{tunnel: portfwdserver.NewTunnelServer()})
	defer client.Close()
	getClient := func() *guestagentclient.GuestAgentClient { return client }

	fw := NewPortForwarder(nil, false, false, nil)
	defer fw.Close()
	fw.ForwardStatic(t.Context(), DialContextToGRPCTunnel(getClient), "unix", hostSocket, guestSocket)

	conn := dialWithRetry(t, hostSocket)
	defer conn.Close()

	payload := []byte("hello from the host")
	_, err := conn.Write(payload)
	assert.NilError(t, err)

	buf := make([]byte, len(payload))
	_, err = io.ReadFull(conn, buf)
	assert.NilError(t, err)
	assert.DeepEqual(t, buf, payload)
}

func TestForwardUnixIsIdempotent(t *testing.T) {
	dir := shortUnixTempDir(t)
	guestSocket := filepath.Join(dir, "guest.sock")
	hostSocket := filepath.Join(dir, "host.sock")

	echoUnixListener(t, guestSocket)

	client := newTestGuestAgentClient(t, &tunnelGuestService{tunnel: portfwdserver.NewTunnelServer()})
	defer client.Close()
	getClient := func() *guestagentclient.GuestAgentClient { return client }

	fw := NewPortForwarder(nil, false, false, nil)
	defer fw.Close()
	dialContext := DialContextToGRPCTunnel(getClient)
	fw.ForwardStatic(t.Context(), dialContext, "unix", hostSocket, guestSocket)
	fw.ForwardStatic(t.Context(), dialContext, "unix", hostSocket, guestSocket)

	conn := dialWithRetry(t, hostSocket)
	defer conn.Close()

	payload := []byte("still works")
	_, err := conn.Write(payload)
	assert.NilError(t, err)

	buf := make([]byte, len(payload))
	_, err = io.ReadFull(conn, buf)
	assert.NilError(t, err)
	assert.DeepEqual(t, buf, payload)
}

func TestForwardUnixDoesNotReplaceExistingHostSocket(t *testing.T) {
	dir := shortUnixTempDir(t)
	guestSocket := filepath.Join(dir, "guest.sock")
	hostSocket := filepath.Join(dir, "host.sock")

	echoUnixListener(t, guestSocket)

	client := newTestGuestAgentClient(t, &tunnelGuestService{tunnel: portfwdserver.NewTunnelServer()})
	defer client.Close()
	getClient := func() *guestagentclient.GuestAgentClient { return client }

	fw := NewPortForwarder(nil, false, false, nil)
	defer fw.Close()
	dialContext := DialContextToGRPCTunnel(getClient)
	fw.ForwardStatic(t.Context(), dialContext, "unix", hostSocket, guestSocket)

	conn := dialWithRetry(t, hostSocket)
	conn.Close()

	// a conflicting target must not unlink the listener
	fw.closableListeners.forwardUnix(t.Context(), dialContext, hostSocket, filepath.Join(dir, "other.sock"))

	conn = dialWithRetry(t, hostSocket)
	defer conn.Close()

	payload := []byte("original target")
	_, err := conn.Write(payload)
	assert.NilError(t, err)

	buf := make([]byte, len(payload))
	_, err = io.ReadFull(conn, buf)
	assert.NilError(t, err)
	assert.DeepEqual(t, buf, payload)
}

func TestForwardUnixClosedOnForwarderClose(t *testing.T) {
	dir := shortUnixTempDir(t)
	guestSocket := filepath.Join(dir, "guest.sock")
	hostSocket := filepath.Join(dir, "host.sock")

	echoUnixListener(t, guestSocket)

	client := newTestGuestAgentClient(t, &tunnelGuestService{tunnel: portfwdserver.NewTunnelServer()})
	defer client.Close()
	getClient := func() *guestagentclient.GuestAgentClient { return client }

	fw := NewPortForwarder(nil, false, false, nil)
	fw.ForwardStatic(t.Context(), DialContextToGRPCTunnel(getClient), "unix", hostSocket, guestSocket)

	conn := dialWithRetry(t, hostSocket)
	conn.Close()

	assert.NilError(t, fw.Close())

	deadline := time.Now().Add(2 * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		_, lastErr = os.Stat(hostSocket)
		if os.IsNotExist(lastErr) {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	assert.Assert(t, os.IsNotExist(lastErr), "host socket %s was not removed after Close", hostSocket)
}

func TestHandleUnixConnectionReturnsOnTunnelFailure(t *testing.T) {
	dir := shortUnixTempDir(t)
	// no listener on the guest socket, so the tunnel fails right after it is opened
	guestSocket := filepath.Join(dir, "missing.sock")

	client := newTestGuestAgentClient(t, &tunnelGuestService{tunnel: portfwdserver.NewTunnelServer()})
	defer client.Close()
	getClient := func() *guestagentclient.GuestAgentClient { return client }

	// The host client never writes or closes, so the handler only returns if a tunnel
	// failure tears down both directions instead of half-closing the host connection.
	hostConn, proxyConn := net.Pipe()
	defer hostConn.Close()

	done := make(chan struct{})
	go func() {
		handleUnixConnection(t.Context(), DialContextToGRPCTunnel(getClient), proxyConn, guestSocket)
		close(done)
	}()

	var returned bool
	select {
	case <-done:
		returned = true
	case <-time.After(5 * time.Second):
	}
	assert.Assert(t, returned, "handleUnixConnection did not return after the tunnel failed")
}
