//go:build darwin && !no_vz

// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package vz

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"sync"
	"syscall"
	"time"

	"github.com/balajiv113/fd"
	"github.com/sirupsen/logrus"
)

func PassFDToUnix(unixSock string) (*os.File, error) {
	unixAddr, err := net.ResolveUnixAddr("unix", unixSock)
	if err != nil {
		return nil, err
	}
	unixConn, err := net.DialUnix("unix", nil, unixAddr)
	if err != nil {
		return nil, err
	}

	server, client, err := createSockPair()
	if err != nil {
		return nil, err
	}
	err = fd.Put(unixConn, server)
	if err != nil {
		return nil, err
	}
	return client, nil
}

func dialQemuConn(ctx context.Context, unixSock string) (*qemuPacketConn, error) {
	var dialer net.Dialer
	conn, err := dialer.DialContext(ctx, "unix", unixSock)
	if err != nil {
		return nil, err
	}
	return &qemuPacketConn{Conn: conn}, nil
}

// DialQemu support connecting to QEMU supported network stack via unix socket.
// Returns os.File, connected dgram connection to be used for vz.
func DialQemu(ctx context.Context, unixSock string) (*os.File, error) {
	qemuConn, err := dialQemuConn(ctx, unixSock)
	if err != nil {
		return nil, err
	}

	server, client, err := createSockPair()
	if err != nil {
		return nil, err
	}
	dgramConn, err := net.FileConn(server)
	if err != nil {
		return nil, err
	}
	vzConn := &packetConn{Conn: dgramConn}

	go forwardPackets(ctx, unixSock, qemuConn, vzConn)

	return client, nil
}

// forwardPackets relays packets between vzConn and qemuConn, reconnecting the
// qemuConn (socket_vmnet) side on failure. vzConn is bound to
// Virtualization.framework at VM creation and cannot be replaced; only qemuConn
// is re-dialed on reconnect.
func forwardPackets(ctx context.Context, unixSock string, qemuConn *qemuPacketConn, vzConn *packetConn) {
	defer vzConn.Close()

	const (
		initialBackoff = 100 * time.Millisecond
		maxBackoff     = 2 * time.Second
	)

	for {
		fatal := forwardUntilError(qemuConn, vzConn)
		if fatal {
			logrus.Error("VZ network socket error, packet forwarding stopped permanently")
			return
		}

		logrus.Warn("VMNET connection lost, reconnecting")

		backoff := initialBackoff
		for {
			if ctx.Err() != nil {
				return
			}

			select {
			case <-ctx.Done():
				return
			case <-time.After(backoff):
			}

			var err error
			qemuConn, err = dialQemuConn(ctx, unixSock)
			if err != nil {
				logrus.WithError(err).Debug("Failed to reconnect to VMNET")
				backoff *= 2
				if backoff > maxBackoff {
					backoff = maxBackoff
				}
				continue
			}

			logrus.Info("VMNET connection re-established, resuming packet forwarding")
			break
		}
	}
}

// forwardUntilError forwards packets between qemuConn and vzConn until an error
// occurs. Returns true if the error is fatal (vzConn failure), false if
// recoverable (qemuConn failure). Always closes qemuConn before returning.
// The goroutine blocked on vzConn.Read may not unblock until the guest sends
// its next packet (ARP, NDP, DHCP — typically within seconds).
func forwardUntilError(qemuConn *qemuPacketConn, vzConn *packetConn) bool {
	errCh := make(chan bool, 2)

	var wg sync.WaitGroup
	wg.Add(2)

	// VZ → VMNET: reads vzConn, writes qemuConn
	go func() {
		defer wg.Done()
		readErr, _ := copyPackets(qemuConn, vzConn)
		errCh <- readErr // readErr=true → vzConn read failed (fatal)
	}()

	// VMNET → VZ: reads qemuConn, writes vzConn
	go func() {
		defer wg.Done()
		readErr, _ := copyPackets(vzConn, qemuConn)
		errCh <- !readErr // !readErr → vzConn write failed (fatal)
	}()

	first := <-errCh
	qemuConn.Close()
	wg.Wait()
	second := <-errCh

	return first || second
}

// maxPacketSize is the maximum Ethernet frame (1514) plus 4-byte QEMU length
// prefix. Used for relay buffers and as a sanity check on incoming frames.
const maxPacketSize = 1518

// copyPackets is like io.Copy but reports whether the error was a read error
// (true) or write error (false), so the caller can distinguish fatal (vzConn)
// from recoverable (qemuConn) failures.
func copyPackets(dst io.Writer, src io.Reader) (readErr bool, err error) {
	buf := make([]byte, maxPacketSize)
	for {
		nr, er := src.Read(buf)
		if nr > 0 {
			if _, ew := dst.Write(buf[:nr]); ew != nil {
				return false, ew
			}
		}
		if er != nil {
			return true, er
		}
	}
}

// qemuPacketConn converts raw network packet to a QEMU supported network packet.
type qemuPacketConn struct {
	net.Conn
}

// Read reads a QEMU packet and returns the contained raw packet.  Returns (len,
// nil) if a packet was read, and (0, err) on error. Errors means the protocol
// is broken and the socket must be closed.
func (c *qemuPacketConn) Read(b []byte) (n int, err error) {
	var size uint32
	if err := binary.Read(c.Conn, binary.BigEndian, &size); err != nil {
		// Likely connection closed by peer.
		return 0, err
	}
	if size > uint32(len(b)) {
		return 0, fmt.Errorf("packet size %d exceeds buffer %d", size, len(b))
	}
	return io.ReadFull(c.Conn, b[:size])
}

// Write writes a QEMU packet containing the raw packet. Returns (len(b), nil)
// if a packet was written, and (0, err) if a packet was not fully written.
// Errors means the protocol is broken and the socket must be closed.
func (c *qemuPacketConn) Write(b []byte) (int, error) {
	size := len(b)
	header := uint32(size)
	if err := binary.Write(c.Conn, binary.BigEndian, header); err != nil {
		return 0, err
	}

	for len(b) != 0 {
		n, err := c.Conn.Write(b)
		if err != nil {
			return 0, err
		}
		b = b[n:]
	}
	return size, nil
}

// Testing show that retries are very rare (e.g 24 of 62,499,008 packets) and
// requires 1 or 2 retries to complete the write. A 100 microseconds sleep loop
// consumes about 4% CPU on M1 Pro.
const writeRetryDelay = 100 * time.Microsecond

// packetConn handles ENOBUFS errors when writing to unixgram socket.
type packetConn struct {
	net.Conn
}

// Write writes a packet retrying on ENOBUFS errors.
func (c *packetConn) Write(b []byte) (int, error) {
	var retries uint64
	for {
		n, err := c.Conn.Write(b)
		if n == 0 && err != nil && errors.Is(err, syscall.ENOBUFS) {
			// This is an expected condition on BSD based system. The kernel
			// does not support blocking until buffer space is available.
			// The only way to recover is to retry the call until it
			// succeeds, or drop the packet.
			// Handled in a similar way in gvisor-tap-vsock:
			// https://github.com/containers/gvisor-tap-vsock/issues/367
			time.Sleep(writeRetryDelay)
			retries++
			continue
		}
		if err != nil {
			return 0, err
		}
		if n < len(b) {
			return n, errors.New("incomplete write to unixgram socket")
		}
		if retries > 0 {
			logrus.Debugf("Write completed after %d retries", retries)
		}
		return n, nil
	}
}
