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
	"time"
)

type SerialConn struct {
	serialDevice string
	port         *Port
}

var _ net.Conn = (*SerialConn)(nil)

func DialSerial(serialDevice string) (*SerialConn, error) {
	s, err := openPort(serialDevice)
	if err != nil {
		return nil, err
	}

	return &SerialConn{port: s, serialDevice: serialDevice}, nil
}

func (c *SerialConn) Read(b []byte) (n int, err error) {
	return c.port.Read(b)
}

func (c *SerialConn) Write(b []byte) (n int, err error) {
	return c.port.Write(b)
}

func (c *SerialConn) Close() error {
	// There is no need to close the serial port every time.
	// So just do nothing.
	return nil
}

func (c *SerialConn) LocalAddr() net.Addr {
	return &net.UnixAddr{Name: "virtio-port:" + c.serialDevice, Net: "virtio"}
}

func (c *SerialConn) RemoteAddr() net.Addr {
	return &net.UnixAddr{Name: "qemu-host", Net: "virtio"}
}

func (c *SerialConn) SetDeadline(_ time.Time) error {
	return nil
}

func (c *SerialConn) SetReadDeadline(_ time.Time) error {
	return nil
}

func (c *SerialConn) SetWriteDeadline(_ time.Time) error {
	return nil
}
