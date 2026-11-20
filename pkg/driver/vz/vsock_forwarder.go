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
			p := tcpproxy.DialProxy{
				DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
					return m.dialVsock(ctx, vsockPort)
				},
			}
			p.HandleConn(conn)
		})
		if err != nil {
			logrus.WithError(err).Errorf("Stopped vsock forwarder: %s -> vsock:%d on VM", hostAddress, vsockPort)
		}
	}()
	return nil
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
