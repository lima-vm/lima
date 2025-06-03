// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"io"
	"net"
	"time"
)

type PipeConn struct {
	Reader io.Reader
	Writer io.Writer
}

func (p *PipeConn) Read(b []byte) (n int, err error) {
	return p.Reader.Read(b)
}

func (p *PipeConn) Write(b []byte) (n int, err error) {
	return p.Writer.Write(b)
}

func (p *PipeConn) Close() error {
	var err error
	if closer, ok := p.Reader.(io.Closer); ok {
		err = closer.Close()
	}
	if closer, ok := p.Writer.(io.Closer); ok {
		if closeErr := closer.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}
	return err
}

func (p *PipeConn) LocalAddr() net.Addr {
	return pipeAddr{}
}

func (p *PipeConn) RemoteAddr() net.Addr {
	return pipeAddr{}
}

func (p *PipeConn) SetDeadline(t time.Time) error {
	return nil
}

func (p *PipeConn) SetReadDeadline(t time.Time) error {
	return nil
}

func (p *PipeConn) SetWriteDeadline(t time.Time) error {
	return nil
}

type pipeAddr struct{}

func (pipeAddr) Network() string { return "pipe" }
func (pipeAddr) String() string  { return "pipe" }
