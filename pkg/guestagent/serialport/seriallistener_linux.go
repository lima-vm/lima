// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package serialport

import (
	"net"
	"sync"
	"syscall"

	"golang.org/x/net/netutil"
)

type SerialListener struct {
	mu     sync.Mutex
	conn   *SerialConn
	closed bool
}

func Listen(serialDevice string) (net.Listener, error) {
	c, err := DialSerial(serialDevice)
	if err != nil {
		return nil, &net.OpError{Op: "dial", Net: "virtio", Source: c.LocalAddr(), Addr: nil, Err: err}
	}

	return netutil.LimitListener(&SerialListener{conn: c}, 1), nil
}

func (ln *SerialListener) ok() bool {
	return ln != nil && ln.conn != nil && !ln.closed
}

func (ln *SerialListener) Accept() (net.Conn, error) {
	ln.mu.Lock()
	defer ln.mu.Unlock()

	if !ln.ok() {
		return nil, syscall.EINVAL
	}

	return ln.conn, nil
}

func (ln *SerialListener) Close() error {
	ln.mu.Lock()
	defer ln.mu.Unlock()

	if !ln.ok() {
		return syscall.EINVAL
	}

	if ln.closed {
		return nil
	}
	ln.closed = true

	return nil
}

func (ln *SerialListener) Addr() net.Addr {
	if ln.ok() {
		return ln.conn.LocalAddr()
	}

	return nil
}
