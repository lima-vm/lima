/*
Copyright The Lima Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

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
