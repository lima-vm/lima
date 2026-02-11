// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package portfwdserver

import (
	"context"
	"errors"
	"io"
	"net"
	"time"

	"github.com/inetaf/tcpproxy"
	"github.com/sirupsen/logrus"

	"github.com/lima-vm/lima/v2/pkg/guestagent/api"
)

type TunnelServer struct{}

func NewTunnelServer() *TunnelServer {
	return &TunnelServer{}
}

func (s *TunnelServer) Start(stream api.GuestService_TunnelServer) error {
	ctx := stream.Context()
	// Receive the handshake message to start tunnel
	in, err := stream.Recv()
	if err != nil {
		if errors.Is(err, io.EOF) {
			return nil
		}
		return err
	}

	// We simply forward data form GRPC stream to net.Conn for both tcp and udp. So simple proxy is sufficient
	var dialer net.Dialer
	conn, err := dialer.DialContext(ctx, in.Protocol, in.GuestAddr)
	if err != nil {
		return err
	}
	rw := &GRPCServerRW{stream: stream, id: in.Id, closeCh: make(chan any, 1)}
	go func() {
		<-ctx.Done()
		rw.Close()
	}()

	proxy := tcpproxy.DialProxy{DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
		return conn, nil
	}}
	go proxy.HandleConn(rw)

	// The stream will be closed when this function returns.
	// Wait here until rw.Close(), rw.CloseRead(), or rw.CloseWrite() is called.
	// We can't close rw.closeCh since the calling order of Close* methods is not guaranteed.
	<-rw.closeCh
	logrus.Debugf("closed GRPCServerRW for id: %s", in.Id)

	return nil
}

type GRPCServerRW struct {
	id      string
	stream  api.GuestService_TunnelServer
	closeCh chan any
}

var _ net.Conn = (*GRPCServerRW)(nil)

func (g *GRPCServerRW) Write(p []byte) (n int, err error) {
	err = g.stream.Send(&api.TunnelMessage{Id: g.id, Data: p})
	return len(p), err
}

func (g *GRPCServerRW) Read(p []byte) (n int, err error) {
	in, err := g.stream.Recv()
	if err != nil {
		return 0, err
	}
	copy(p, in.Data)
	return len(in.Data), nil
}

func (g *GRPCServerRW) Close() error {
	logrus.Debugf("closing GRPCServerRW for id: %s", g.id)
	g.closeCh <- struct{}{}
	return nil
}

// By adding CloseRead and CloseWrite methods, GRPCServerRW can work with
// other than containers/gvisor-tap-vsock/pkg/tcpproxy, e.g., inetaf/tcpproxy, bicopy.Bicopy.

func (g *GRPCServerRW) CloseRead() error {
	logrus.Debugf("closing read GRPCServerRW for id: %s", g.id)
	g.closeCh <- struct{}{}
	return nil
}

func (g *GRPCServerRW) CloseWrite() error {
	logrus.Debugf("closing write GRPCServerRW for id: %s", g.id)
	g.closeCh <- struct{}{}
	return nil
}

func (g *GRPCServerRW) LocalAddr() net.Addr {
	return &net.UnixAddr{Name: "grpc", Net: "unixpacket"}
}

func (g *GRPCServerRW) RemoteAddr() net.Addr {
	return &net.UnixAddr{Name: "grpc", Net: "unixpacket"}
}

func (g *GRPCServerRW) SetDeadline(_ time.Time) error {
	return nil
}

func (g *GRPCServerRW) SetReadDeadline(_ time.Time) error {
	return nil
}

func (g *GRPCServerRW) SetWriteDeadline(_ time.Time) error {
	return nil
}
