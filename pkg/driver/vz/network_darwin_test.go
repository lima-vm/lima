//go:build darwin && !no_vz

// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package vz

import (
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"path/filepath"
	"testing"
	"time"

	"gotest.tools/v3/assert"
)

const (
	vmnetMaxPacketSize = 1514
	packetsCount       = 1000
)

func TestDialQemu(t *testing.T) {
	listener, err := listenUnix(t.TempDir())
	assert.NilError(t, err)
	defer listener.Close()
	t.Logf("Listening at %q", listener.Addr())

	errc := make(chan error, 2)

	// Start the fake vmnet server.
	go func() {
		t.Log("Fake vmnet started")
		errc <- serveOneClient(listener)
		t.Log("Fake vmnet finished")
	}()

	// Connect to the fake vmnet server.
	client, err := DialQemu(t.Context(), listener.Addr().String())
	assert.NilError(t, err)
	t.Log("Connected to fake vment server")

	dgramConn, err := net.FileConn(client)
	assert.NilError(t, err)

	vzConn := packetConn{Conn: dgramConn}
	defer vzConn.Close()

	go func() {
		t.Log("Sender started")
		buf := make([]byte, vmnetMaxPacketSize)
		for i := range vmnetMaxPacketSize {
			buf[i] = 0x55
		}

		// data packet format:
		//     0-4		packet number
		//     4-1514	0x55 ...
		for i := range packetsCount {
			binary.BigEndian.PutUint32(buf, uint32(i))
			if _, err := vzConn.Write(buf); err != nil {
				errc <- err
				return
			}
		}
		t.Logf("Sent %d data packets", packetsCount)

		// quit packet format:
		//     0-4:     "quit"
		copy(buf[:4], "quit")
		if _, err := vzConn.Write(buf[:4]); err != nil {
			errc <- err
			return
		}

		errc <- nil
		t.Log("Sender finished")
	}()

	// Read and verify packets to the server.

	buf := make([]byte, vmnetMaxPacketSize)

	t.Log("Receiving and verifying data packets...")
	for i := range packetsCount {
		n, err := vzConn.Read(buf)
		assert.NilError(t, err)
		assert.Assert(t, n >= vmnetMaxPacketSize, "unexpected number of bytes")

		number := binary.BigEndian.Uint32(buf[:4])
		assert.Equal(t, number, uint32(i), "unexpected packet")

		for j := 4; j < vmnetMaxPacketSize; j++ {
			assert.Equal(t, buf[j], byte(0x55), "unexpected byte at offset %d", j)
		}
	}
	t.Logf("Received and verified %d data packets", packetsCount)

	for range 2 {
		err := <-errc
		assert.NilError(t, err)
	}
}

func TestForwardPacketsReconnect(t *testing.T) {
	listener, err := listenUnix(t.TempDir())
	assert.NilError(t, err)
	defer listener.Close()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	// Start first fake vmnet server.
	server1Done := make(chan error, 1)
	go func() {
		server1Done <- serveOneClient(listener)
	}()

	client, err := DialQemu(ctx, listener.Addr().String())
	assert.NilError(t, err)

	dgramConn, err := net.FileConn(client)
	assert.NilError(t, err)
	vzClient := packetConn{Conn: dgramConn}
	defer vzClient.Close()

	sendRecv := func(seq uint32) {
		t.Helper()
		buf := make([]byte, vmnetMaxPacketSize)
		binary.BigEndian.PutUint32(buf, seq)
		_, err := vzClient.Write(buf)
		assert.NilError(t, err)
		n, err := vzClient.Read(buf)
		assert.NilError(t, err)
		assert.Assert(t, n >= 4)
		assert.Equal(t, binary.BigEndian.Uint32(buf[:4]), seq)
	}

	// Verify packets through first connection.
	for i := range 10 {
		sendRecv(uint32(i))
	}
	t.Log("First connection: 10 packets verified")

	// Tell first server to quit, closing its connection.
	quit := make([]byte, 4)
	copy(quit, "quit")
	_, err = vzClient.Write(quit)
	assert.NilError(t, err)
	assert.NilError(t, <-server1Done)
	t.Log("First server closed")

	// Start second fake vmnet server for the reconnect.
	server2Done := make(chan error, 1)
	go func() {
		server2Done <- serveOneClient(listener)
	}()

	// Send a sacrificial packet to unblock the VZ→VMNET goroutine that may
	// be blocked reading from vzConn. This packet will be lost during the
	// reconnect window — expected behavior.
	sacrificial := make([]byte, 4)
	binary.BigEndian.PutUint32(sacrificial, 0xFFFFFFFF)
	_, err = vzClient.Write(sacrificial)
	assert.NilError(t, err)

	// Wait for reconnect (100ms backoff + dial).
	time.Sleep(300 * time.Millisecond)

	// Verify packets through second connection.
	for i := 10; i < 20; i++ {
		sendRecv(uint32(i))
	}
	t.Log("Second connection: 10 packets verified after reconnect")

	// Clean up second server.
	_, err = vzClient.Write(quit)
	assert.NilError(t, err)
	assert.NilError(t, <-server2Done)
}

// serveOneClient accepts one client and echo back received packets until a
// "quit" packet is sent.
func serveOneClient(listener *net.UnixListener) error {
	conn, err := listener.Accept()
	if err != nil {
		return err
	}
	qemuConn := qemuPacketConn{Conn: conn}
	defer qemuConn.Close()

	buf := make([]byte, vmnetMaxPacketSize)
	for {
		nr, err := qemuConn.Read(buf)
		if err != nil {
			return err
		}
		if string(buf[:4]) == "quit" {
			return nil
		}
		nw, err := qemuConn.Write(buf[:nr])
		if err != nil {
			return err
		}
		if nw != nr {
			return fmt.Errorf("incomplete write: expected: %d, wrote: %d", nr, nw)
		}
	}
}

// listenUnix creates and listen to unix socket under dir
func listenUnix(dir string) (*net.UnixListener, error) {
	sock := filepath.Join(dir, "sock")
	addr, err := net.ResolveUnixAddr("unix", sock)
	if err != nil {
		return nil, err
	}
	return net.ListenUnix("unix", addr)
}
