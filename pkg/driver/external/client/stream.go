// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"errors"
	"io"
	"net"
	"sync"
	"time"

	pb "github.com/lima-vm/lima/pkg/driver/external"
)

type streamConn struct {
	stream    pb.Driver_GuestAgentConnClient
	readBuf   []byte
	readMu    sync.Mutex
	closeCh   chan struct{}
	closeOnce sync.Once
	closed    bool
}

func streamToConn(stream pb.Driver_GuestAgentConnClient) *streamConn {
	return &streamConn{
		stream:  stream,
		closeCh: make(chan struct{}),
	}
}

func (c *streamConn) Read(b []byte) (int, error) {
	c.readMu.Lock()
	defer c.readMu.Unlock()

	if c.closed {
		return 0, io.EOF
	}

	// Use any leftover data first
	if len(c.readBuf) > 0 {
		n := copy(b, c.readBuf)
		c.readBuf = c.readBuf[n:]
		return n, nil
	}

	// Receive from stream
	msg, err := c.stream.Recv()
	if err != nil {
		if err == io.EOF {
			c.closed = true
			return 0, io.EOF
		}
		return 0, err
	}

	// Copy data to buffer
	n := copy(b, msg.NetConn)
	if n < len(msg.NetConn) {
		// Store remaining data for next read
		c.readBuf = make([]byte, len(msg.NetConn)-n)
		copy(c.readBuf, msg.NetConn[n:])
	}

	return n, nil
}

func (c *streamConn) Write(b []byte) (int, error) {
	return 0, errors.New("write not supported on read-only stream connection")
}

func (c *streamConn) Close() error {
	c.closeOnce.Do(func() {
		c.readMu.Lock()
		c.closed = true
		c.readMu.Unlock()
		close(c.closeCh)
		c.stream.CloseSend()
	})
	return nil
}

func (c *streamConn) LocalAddr() net.Addr  { return &grpcAddr{} }
func (c *streamConn) RemoteAddr() net.Addr { return &grpcAddr{} }

func (c *streamConn) SetDeadline(t time.Time) error      { return nil }
func (c *streamConn) SetReadDeadline(t time.Time) error  { return nil }
func (c *streamConn) SetWriteDeadline(t time.Time) error { return nil }

type grpcAddr struct{}

func (grpcAddr) Network() string { return "grpc" }
func (grpcAddr) String() string  { return "grpc-stream" }
