//go:build darwin && !no_vz

// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package vz

import (
	"context"
	"fmt"
	"net"

	"github.com/containers/gvisor-tap-vsock/pkg/tcpproxy"
	"github.com/sirupsen/logrus"
)

func (m *virtualMachineWrapper) startVsockForwarder(ctx context.Context, vsockPort, hostPort uint32) error {
	// Test if the vsock port is open
	conn, err := m.dialVsock(ctx, vsockPort)
	if err != nil {
		return err
	}
	conn.Close()
	// Start listening on localhost:hostPort and forward to vsock:vsockPort
	var lc net.ListenConfig
	l, err := lc.Listen(ctx, "tcp", fmt.Sprintf("127.0.0.1:%d", hostPort))
	if err != nil {
		return err
	}
	logrus.Infof("started vsock forwarder: localhost:%d -> vsock:%d on VM", hostPort, vsockPort)
	go func() {
		defer l.Close()
		for {
			conn, err := l.Accept()
			if err != nil {
				logrus.WithError(err).Errorf("vsock forwarder accept error: %v", err)
				return
			}
			p := tcpproxy.DialProxy{
				DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
					return m.dialVsock(ctx, vsockPort)
				},
			}
			go p.HandleConn(conn)
			select {
			case <-ctx.Done():
				return
			default:
				continue
			}
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
