// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

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
