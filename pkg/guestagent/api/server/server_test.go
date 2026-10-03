// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package server

import (
	"errors"
	"net"
	"testing"

	"gotest.tools/v3/assert"
)

type fakeConn struct {
	net.Conn
	writeErr error
	closed   bool
}

func (c *fakeConn) Write(b []byte) (int, error) {
	if c.writeErr != nil {
		return 0, c.writeErr
	}
	return len(b), nil
}

func (c *fakeConn) Close() error {
	c.closed = true
	return nil
}

type fakeListener struct {
	net.Listener
	conn net.Conn
}

func (l *fakeListener) Accept() (net.Conn, error) {
	return l.conn, nil
}

func TestCloseOnWriteErrorListener(t *testing.T) {
	t.Run("write succeeds", func(t *testing.T) {
		inner := &fakeConn{}
		lis := &closeOnWriteErrorListener{Listener: &fakeListener{conn: inner}}
		conn, err := lis.Accept()
		assert.NilError(t, err)
		n, err := conn.Write([]byte("hello"))
		assert.NilError(t, err)
		assert.Equal(t, n, 5)
		assert.Assert(t, !inner.closed)
	})
	t.Run("write fails", func(t *testing.T) {
		writeErr := errors.New("write failed")
		inner := &fakeConn{writeErr: writeErr}
		lis := &closeOnWriteErrorListener{Listener: &fakeListener{conn: inner}}
		conn, err := lis.Accept()
		assert.NilError(t, err)
		_, err = conn.Write([]byte("hello"))
		assert.ErrorIs(t, err, writeErr)
		assert.Assert(t, inner.closed, "connection must be closed after a write error")
	})
}
