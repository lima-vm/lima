// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package server

// import (
// 	"io"
// 	"net"
// 	"sync"
// 	"time"
// )

// type PipeConn struct {
// 	Reader io.Reader
// 	Writer io.Writer
// 	Closer io.Closer
// }

// func (p *PipeConn) Read(b []byte) (n int, err error) {
// 	return p.Reader.Read(b)
// }

// func (p *PipeConn) Write(b []byte) (n int, err error) {
// 	return p.Writer.Write(b)
// }

// func (p *PipeConn) Close() error {
// 	return p.Closer.Close()
// }

// func (p *PipeConn) LocalAddr() net.Addr {
// 	return pipeAddr{}
// }

// func (p *PipeConn) RemoteAddr() net.Addr {
// 	return pipeAddr{}
// }

// func (p *PipeConn) SetDeadline(t time.Time) error {
// 	return nil
// }

// func (p *PipeConn) SetReadDeadline(t time.Time) error {
// 	return nil
// }

// func (p *PipeConn) SetWriteDeadline(t time.Time) error {
// 	return nil
// }

// type pipeAddr struct{}

// func (pipeAddr) Network() string { return "pipe" }
// func (pipeAddr) String() string  { return "pipe" }

// type PipeListener struct {
// 	conn     net.Conn
// 	connSent bool
// 	mu       sync.Mutex
// 	closed   bool
// }

// func NewPipeListener(conn net.Conn) *PipeListener {
// 	return &PipeListener{
// 		conn:     conn,
// 		connSent: false,
// 		closed:   false,
// 	}
// }

// func (l *PipeListener) Accept() (net.Conn, error) {
// 	l.mu.Lock()
// 	defer l.mu.Unlock()

// 	if l.closed {
// 		return nil, net.ErrClosed
// 	}

// 	if l.connSent {
// 		select {}
// 	}

// 	l.connSent = true
// 	return l.conn, nil
// }

// func (l *PipeListener) Close() error {
// 	l.mu.Lock()
// 	defer l.mu.Unlock()

// 	if !l.closed {
// 		l.closed = true
// 	}
// 	return nil
// }

// func (l *PipeListener) Addr() net.Addr {
// 	return pipeAddr{}
// }
