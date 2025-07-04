// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package portfwdserver

import (
	"context"
	"errors"
	"io"
	"net"
	"time"

	"github.com/lima-vm/lima/pkg/guestagent/api"
	"github.com/lima-vm/lima/pkg/tcpproxy"
)

type TunnelServer struct{}

func NewTunnelServer() *TunnelServer {
	return &TunnelServer{}
}

func (s *TunnelServer) Start(stream api.GuestService_TunnelServer) error {
	// Receive the handshake message to start tunnel
	in, err := stream.Recv()
	if err != nil {
		if errors.Is(err, io.EOF) {
			return nil
		}
		return err
	}

	// We simply forward data form GRPC stream to net.Conn for both tcp and udp. So simple proxy is sufficient
	conn, err := net.Dial(in.Protocol, in.GuestAddr)
	if err != nil {
		return err
	}
	rw := &GRPCServerRW{stream: stream, id: in.Id}
	proxy := tcpproxy.DialProxy{DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
		return conn, nil
	}}
	proxy.HandleConn(rw)
	return nil
}

type GRPCServerRW struct {
	id     string
	stream api.GuestService_TunnelServer
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
