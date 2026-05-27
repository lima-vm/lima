//go:build darwin && !no_vz

// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package vz

import (
	"context"
	"encoding/binary"
	"errors"
	"io"
	"math"
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
// qemuConn (socket_vmnet) side on failure. vzConn is the local end of a DGRAM
// socketpair whose other end is bound to Virtualization.framework at VM creation
// and cannot be changed. Only qemuConn is replaced on reconnect.
//
// During reconnect, packets from the guest buffer in the socketpair kernel buffer
// (4MB). If the buffer fills, the guest gets ENOBUFS (packet drops), which is
// normal for a brief network interruption and recovered by TCP retransmit / ARP
// retry. Packets from the network side are lost while disconnected.
func forwardPackets(ctx context.Context, unixSock string, qemuConn *qemuPacketConn, vzConn *packetConn) {
	defer vzConn.Close()

	const (
		initialBackoff = 100 * time.Millisecond
		maxBackoff     = 30 * time.Second
		backoffFactor  = 2.0
	)

	for {
		fatal := forwardUntilError(qemuConn, vzConn)
		if fatal {
			logrus.Error("VZ network socket error, packet forwarding stopped permanently")
			return
		}

		backoff := initialBackoff
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			logrus.Warnf("VMNET connection lost, reconnecting in %v", backoff)

			select {
			case <-ctx.Done():
				return
			case <-time.After(backoff):
			}

			var err error
			qemuConn, err = dialQemuConn(ctx, unixSock)
			if err != nil {
				logrus.WithError(err).Warn("Failed to reconnect to VMNET")
				backoff = time.Duration(math.Min(float64(backoff)*backoffFactor, float64(maxBackoff)))
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
//
// Teardown: closing qemuConn forces the goroutine blocked on qemuConn I/O to
// exit immediately. The goroutine blocked on vzConn.Read may not unblock until
// the guest sends its next packet, since vzConn must stay open for reuse across
// reconnects. In practice guests generate periodic traffic (ARP, NDP, DHCP)
// within seconds. An interruptible copy or read-deadline approach was considered
// but rejected: SetReadDeadline on the DGRAM vzConn would need to be armed and
// disarmed on every reconnect cycle and risks false-positive timeouts during
// normal forwarding. Waiting for the next guest packet is simpler and adds no
// overhead to the normal forwarding path.
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

// copyPackets copies packets from src to dst, returning whether the error was a
// read error (true) or write error (false). This is used instead of io.Copy
// because io.Copy does not distinguish read errors from write errors, and the
// reconnect logic needs that distinction: a qemuConn (socket_vmnet) error is
// recoverable via reconnect, while a vzConn (VZ socketpair) error is fatal.
// The copy loop is the same read-buf-write-buf loop io.Copy uses (neither
// qemuPacketConn nor packetConn implements ReaderFrom/WriterTo).
func copyPackets(dst io.Writer, src io.Reader) (readErr bool, err error) {
	buf := make([]byte, 32*1024)
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
