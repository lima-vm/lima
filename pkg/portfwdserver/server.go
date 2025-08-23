// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package portfwdserver

import (
	"context"
	"errors"
	"io"
	"net"
	"os"
	"strings"
	"time"

	"github.com/containers/gvisor-tap-vsock/pkg/tcpproxy"

	"github.com/lima-vm/lima/v2/pkg/bicopy"
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
	rw := &GRPCServerRW{stream: stream, id: in.Id}

	// FIXME: consolidate bicopy and tcpproxy into one
	//
	// The bicopy package does not seem to work with `w3m -dump`:
	// https://github.com/lima-vm/lima/issues/3685
	//
	// However, the tcpproxy package can't pass the CI for WSL2 (experimental):
	// https://github.com/lima-vm/lima/pull/3686#issuecomment-3034842616
	if wsl2, _ := seemsWSL2(); wsl2 {
		bicopy.Bicopy(rw, conn, nil)
	} else {
		proxy := tcpproxy.DialProxy{DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
			return conn, nil
		}}
		proxy.HandleConn(rw)
	}

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

// seemsWSL2 returns whether lima.env contains LIMA_CIDATA_VMTYPE=wsl2 .
// This is a temporary workaround and has to be removed.
func seemsWSL2() (bool, error) {
	b, err := os.ReadFile("/mnt/lima-cidata/lima.env")
	if err != nil {
		return false, err
	}
	return strings.Contains(string(b), "LIMA_CIDATA_VMTYPE=wsl2"), nil
}
