// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package grpchijack

import (
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	pb "github.com/lima-vm/lima/pkg/driver/external"
	"google.golang.org/grpc"
)

type streamConn struct {
	stream     pb.Driver_GuestAgentConnClient
	readBuf    []byte
	lastBuf    []byte
	readMu     sync.Mutex
	writeMu    sync.Mutex
	closeCh    chan struct{}
	closedOnce sync.Once
	closed     bool
}

func StreamToConn(stream pb.Driver_GuestAgentConnClient) *streamConn {
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

	if c.lastBuf != nil {
		n := copy(b, c.lastBuf)
		c.lastBuf = c.lastBuf[n:]
		if len(c.lastBuf) == 0 {
			c.lastBuf = nil
		}
		return n, nil
	}

	msg := new(pb.BytesMessage)
	msg, err := c.stream.Recv()
	if err != nil {
		c.closed = true
		if err == io.EOF {
			return 0, io.EOF
		}
		return 0, fmt.Errorf("stream receive error: %w", err)
	}

	n := copy(b, msg.Data)
	if n < len(msg.Data) {
		c.readBuf = make([]byte, len(msg.Data)-n)
		copy(c.readBuf, msg.Data[n:])
	}

	return n, nil
}

func (c *streamConn) Write(b []byte) (int, error) {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	if c.closed {
		return 0, errors.New("connection closed")
	}

	err := c.stream.Send(&pb.BytesMessage{Data: b})
	if err != nil {
		c.closed = true
		return 0, fmt.Errorf("stream send error: %w", err)
	}

	return len(b), nil
}

func (c *streamConn) Close() error {
	c.closedOnce.Do(func() {
		defer func() {
			close(c.closeCh)
		}()

		if cs, ok := c.stream.(grpc.ClientStream); ok {
			c.writeMu.Lock()
			err := cs.CloseSend()
			c.writeMu.Unlock()
			if err != nil {
				return
			}
		}

		c.readMu.Lock()
		for {
			m := new(pb.BytesMessage)
			m.Data = c.readBuf
			err := c.stream.RecvMsg(m)
			if err != nil {
				if !errors.Is(err, io.EOF) {
					c.readMu.Unlock()
					return
				}
				err = nil
				break
			}
			c.readBuf = m.Data[:cap(m.Data)]
			c.lastBuf = append(c.lastBuf, c.readBuf...)
		}
		c.readMu.Unlock()
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
