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

// DialQemu support connecting to QEMU supported network stack via unix socket.
// Returns os.File, connected dgram connection to be used for vz.
func DialQemu(ctx context.Context, unixSock string) (*os.File, error) {
	var dialer net.Dialer
	conn, err := dialer.DialContext(ctx, "unix", unixSock)
	if err != nil {
		return nil, err
	}

	server, client, err := createSockPair()
	if err != nil {
		conn.Close()
		return nil, err
	}
	dgramConn, err := net.FileConn(server)
	if err != nil {
		conn.Close()
		return nil, err
	}
	vzConn := &packetConn{Conn: dgramConn}

	fwdCtx, fwdCancel := context.WithCancel(ctx)
	qemuConn := newQemuPacketConn(fwdCtx, unixSock, conn)

	go forwardPackets(fwdCancel, qemuConn, vzConn)

	return client, nil
}

// forwardPackets relays packets between vzConn and qemuConn. Reconnection of
// the qemuConn side is handled internally by qemuPacketConn; this function
// just runs io.Copy in both directions until a fatal error occurs.
func forwardPackets(cancel context.CancelFunc, qemuConn *qemuPacketConn, vzConn *packetConn) {
	done := make(chan struct{}, 2)

	go func() {
		io.Copy(qemuConn, vzConn)
		done <- struct{}{}
	}()

	go func() {
		io.Copy(vzConn, qemuConn)
		done <- struct{}{}
	}()

	<-done
	if !qemuConn.done() {
		logrus.Error("VZ network socket error, packet forwarding stopped permanently")
	}
	cancel()
	vzConn.Close()
	qemuConn.Close()
	<-done
}

// qemuPacketConn relays length-prefixed Ethernet frames over a unix stream
// socket to socket_vmnet, reconnecting automatically on connection failure.
type qemuPacketConn struct {
	mu        sync.Mutex
	cond      *sync.Cond
	conn      net.Conn
	gen       uint64
	reconnect chan uint64
	ctx       context.Context
	sock      string
}

func newQemuPacketConn(ctx context.Context, sock string, conn net.Conn) *qemuPacketConn {
	c := &qemuPacketConn{
		conn:      conn,
		reconnect: make(chan uint64, 1),
		ctx:       ctx,
		sock:      sock,
	}
	c.cond = sync.NewCond(&c.mu)
	go c.reconnectLoop()
	go func() {
		<-ctx.Done()
		c.cond.Broadcast()
	}()
	return c
}

// getConn returns the current connection and its generation. Blocks while a
// reconnect is in progress. Returns ok=false on shutdown.
func (c *qemuPacketConn) getConn() (conn net.Conn, gen uint64, ok bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for c.conn == nil {
		if c.ctx.Err() != nil {
			return nil, c.gen, false
		}
		c.cond.Wait()
	}
	return c.conn, c.gen, true
}

// waitReconnect signals the reconnect goroutine and blocks until a new
// connection is available. Returns false on shutdown.
func (c *qemuPacketConn) waitReconnect(gen uint64) bool {
	select {
	case c.reconnect <- gen:
	default:
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	for c.gen == gen {
		if c.ctx.Err() != nil {
			return false
		}
		c.cond.Wait()
	}
	return c.conn != nil
}

func (c *qemuPacketConn) reconnectLoop() {
	const (
		initialBackoff = 100 * time.Millisecond
		maxBackoff     = 2 * time.Second
	)

	for {
		select {
		case <-c.ctx.Done():
			c.shutdown()
			return
		case failedGen := <-c.reconnect:
			c.mu.Lock()
			if c.gen != failedGen {
				c.mu.Unlock()
				continue
			}
			old := c.conn
			c.conn = nil
			c.mu.Unlock()

			if old != nil {
				old.Close()
			}

			logrus.Warn("VMNET connection lost, reconnecting")

			var dialer net.Dialer
			backoff := initialBackoff
			for {
				select {
				case <-c.ctx.Done():
					c.shutdown()
					return
				case <-time.After(backoff):
				}

				conn, err := dialer.DialContext(c.ctx, "unix", c.sock)
				if err != nil {
					logrus.WithError(err).Debug("Failed to reconnect to VMNET")
					backoff *= 2
					if backoff > maxBackoff {
						backoff = maxBackoff
					}
					continue
				}

				if c.ctx.Err() != nil {
					conn.Close()
					c.shutdown()
					return
				}

				c.mu.Lock()
				c.conn = conn
				c.gen++
				c.cond.Broadcast()
				c.mu.Unlock()

				logrus.Info("VMNET connection re-established")
				break
			}
		}
	}
}

func (c *qemuPacketConn) done() bool {
	return c.ctx.Err() != nil
}

func (c *qemuPacketConn) shutdown() {
	c.mu.Lock()
	if c.conn != nil {
		c.conn.Close()
	}
	c.conn = nil
	c.gen++
	c.cond.Broadcast()
	c.mu.Unlock()
}

func (c *qemuPacketConn) Read(b []byte) (int, error) {
	for {
		conn, gen, ok := c.getConn()
		if !ok {
			return 0, c.ctx.Err()
		}
		n, err := readPacket(conn, b)
		if err == nil {
			return n, nil
		}
		if !c.waitReconnect(gen) {
			return 0, err
		}
	}
}

func (c *qemuPacketConn) Write(b []byte) (int, error) {
	for {
		conn, gen, ok := c.getConn()
		if !ok {
			return 0, c.ctx.Err()
		}
		err := writePacket(conn, b)
		if err == nil {
			return len(b), nil
		}
		if !c.waitReconnect(gen) {
			return 0, err
		}
	}
}

func (c *qemuPacketConn) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn != nil {
		err := c.conn.Close()
		c.conn = nil
		return err
	}
	return nil
}

func readPacket(conn net.Conn, b []byte) (int, error) {
	var size uint32
	if err := binary.Read(conn, binary.BigEndian, &size); err != nil {
		return 0, err
	}
	if size > uint32(len(b)) {
		return 0, fmt.Errorf("packet size %d exceeds buffer %d", size, len(b))
	}
	return io.ReadFull(conn, b[:size])
}

func writePacket(conn net.Conn, b []byte) error {
	if err := binary.Write(conn, binary.BigEndian, uint32(len(b))); err != nil {
		return err
	}
	for len(b) != 0 {
		n, err := conn.Write(b)
		if err != nil {
			return err
		}
		b = b[n:]
	}
	return nil
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
