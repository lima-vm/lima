//go:build darwin && !no_vz

// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package vz

import (
	"context"
	"net"

	"github.com/inetaf/tcpproxy"
	"github.com/sirupsen/logrus"

	"github.com/lima-vm/lima/v2/pkg/acceptloop"
)

func (m *virtualMachineWrapper) startVsockForwarder(ctx context.Context, vsockPort uint32, hostAddress string) error {
	// Test if the vsock port is open
	conn, err := m.dialVsock(ctx, vsockPort)
	if err != nil {
		return err
	}
	conn.Close()
	// Start listening on localhost:hostPort and forward to vsock:vsockPort
	_, _, err = net.SplitHostPort(hostAddress)
	if err != nil {
		return err
	}
	var lc net.ListenConfig
	l, err := lc.Listen(ctx, "tcp", hostAddress)
	if err != nil {
		return err
	}
	logrus.Infof("Started vsock forwarder: %s -> vsock:%d on VM", hostAddress, vsockPort)
	go func() {
		err := acceptloop.Serve(ctx, l, func(conn net.Conn) {
			p := newVsockProxy(func() (net.Conn, error) {
				return m.dialVsock(ctx, vsockPort)
			})
			p.HandleConn(conn)
		})
		if err != nil {
			logrus.WithError(err).Errorf("Stopped vsock forwarder: %s -> vsock:%d on VM", hostAddress, vsockPort)
		}
	}()
	return nil
}

// newVsockProxy returns a proxy from a TCP connection to the vsock connection opened by dial.
func newVsockProxy(dial func() (net.Conn, error)) *tcpproxy.DialProxy {
	return &tcpproxy.DialProxy{
		DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
			conn, err := dial()
			if err != nil {
				return nil, err
			}
			return closeWriteConn{conn}, nil
		},
	}
}

// closeWriteConn closes the whole connection on CloseWrite.
//
// tcpproxy waits for both directions to end, and when one of them reaches EOF, it only
// shuts down the write side of the other connection. vz.VirtioSocketConnection has no
// CloseWrite, so when an ssh client goes away without an ssh disconnect (for example,
// when it is killed), nothing reaches the guest: the session stays open in the guest,
// and the proxy keeps both connections. The forwarder only carries ssh, which does not
// need a half-close.
type closeWriteConn struct {
	net.Conn
}

func (c closeWriteConn) CloseWrite() error {
	return c.Close()
}

func (m *virtualMachineWrapper) dialVsock(_ context.Context, port uint32) (conn net.Conn, err error) {
	for _, socket := range m.SocketDevices() {
		conn, err = socket.Connect(port)
		if err == nil {
			return conn, nil
		}
	}
	return nil, err
}
