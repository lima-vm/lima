// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package portfwdserver

import (
	"context"
	"errors"
	"io"
	"net"
	"sync"
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
	// The tunnel is unusable once this function returns, so close the dialed
	// connection here as well. This both releases the FD even if the proxy
	// goroutine is still blocked reading from the guest, and unblocks that
	// read so the goroutine can finish.
	defer conn.Close()
	rw := &GRPCServerRW{stream: stream, id: in.Id, closeCh: make(chan any)}
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
	<-rw.closeCh
	logrus.Debugf("closed GRPCServerRW for id: %s", in.Id)

	return nil
}

type GRPCServerRW struct {
	id      string
	stream  api.GuestService_TunnelServer
	closeCh chan any
	// closeOnce guards closeCh. Close, CloseRead, and CloseWrite may be
	// called in any order and multiple times (e.g., tcpproxy calls
	// CloseRead/CloseWrite from each copy direction and then Close), so
	// they must be idempotent and must never block; otherwise the proxy
	// goroutine gets stuck and never closes the dialed guest connection,
	// leaking one FD per forwarded connection.
	closeOnce sync.Once

	// rxBuf holds bytes received from the stream that did not fit in the
	// buffer passed to the previous Read call.
	rxBuf []byte
}

var _ net.Conn = (*GRPCServerRW)(nil)

func (g *GRPCServerRW) Write(p []byte) (n int, err error) {
	err = g.stream.Send(&api.TunnelMessage{Id: g.id, Data: p})
	return len(p), err
}

func (g *GRPCServerRW) Read(p []byte) (n int, err error) {
	if len(g.rxBuf) == 0 {
		in, err := g.stream.Recv()
		if err != nil {
			return 0, err
		}
		g.rxBuf = in.Data
	}
	n = copy(p, g.rxBuf)
	g.rxBuf = g.rxBuf[n:]
	return n, nil
}

func (g *GRPCServerRW) Close() error {
	logrus.Debugf("closing GRPCServerRW for id: %s", g.id)
	g.closeOnce.Do(func() { close(g.closeCh) })
	return nil
}

// By adding CloseRead and CloseWrite methods, GRPCServerRW can work with
// other than containers/gvisor-tap-vsock/pkg/tcpproxy, e.g., inetaf/tcpproxy, bicopy.Bicopy.

func (g *GRPCServerRW) CloseRead() error {
	logrus.Debugf("closing read GRPCServerRW for id: %s", g.id)
	g.closeOnce.Do(func() { close(g.closeCh) })
	return nil
}

func (g *GRPCServerRW) CloseWrite() error {
	logrus.Debugf("closing write GRPCServerRW for id: %s", g.id)
	g.closeOnce.Do(func() { close(g.closeCh) })
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
