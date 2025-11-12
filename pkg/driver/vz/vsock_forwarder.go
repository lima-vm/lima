//go:build darwin && !no_vz

// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package vz

import (
	"context"
	"errors"
	"net"

	"github.com/containers/gvisor-tap-vsock/pkg/tcpproxy"
	"github.com/sirupsen/logrus"

	"github.com/lima-vm/lima/v2/pkg/limatype"
	"github.com/lima-vm/lima/v2/pkg/sshutil"
)

func (m *virtualMachineWrapper) startVsockForwarder(ctx context.Context, vsockPort uint32, hostAddress string) error {
	// Start listening on localhost:hostPort and forward to vsock:vsockPort
	_, _, err := net.SplitHostPort(hostAddress)
	if err != nil {
		return err
	}
	var lc net.ListenConfig
	l, err := lc.Listen(ctx, "tcp", hostAddress)
	if err != nil {
		return err
	}
	go func() {
		<-ctx.Done()
		l.Close()
	}()
	logrus.Infof("Started vsock forwarder: %s -> vsock:%d on VM", hostAddress, vsockPort)
	go func() {
		defer l.Close()
		for {
			conn, err := l.Accept()
			if err != nil {
				if errors.Is(err, net.ErrClosed) {
					return
				}
				logrus.WithError(err).Errorf("vsock forwarder accept error: %v", err)
			} else {
				p := tcpproxy.DialProxy{
					DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
						return m.dialVsock(ctx, vsockPort)
					},
				}
				go p.HandleConn(conn)
			}
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

func (m *virtualMachineWrapper) checkSSHOverVsockAvailable(ctx context.Context, inst *limatype.Instance) error {
	return sshutil.WaitSSHReady(ctx, func(ctx context.Context) (net.Conn, error) {
		return m.dialVsock(ctx, uint32(22))
	}, "vsock:22", *inst.Config.User.Name, 1)
}
