// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package portfwd

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync/atomic"
	"time"

	"github.com/containers/gvisor-tap-vsock/pkg/services/forwarder"
	"github.com/inetaf/tcpproxy"
	"github.com/sirupsen/logrus"

	"github.com/lima-vm/lima/v2/pkg/guestagent/api"
	guestagentclient "github.com/lima-vm/lima/v2/pkg/guestagent/api/client"
)

func HandleTCPConnection(_ context.Context, dialContext func(ctx context.Context, network string, addr string) (net.Conn, error), conn net.Conn, guestAddr string) {
	proxy := tcpproxy.DialProxy{Addr: guestAddr, DialContext: dialContext}
	proxy.HandleConn(conn)
	logrus.Debugf("tcp proxy for guestAddr: %s closed", guestAddr)
}

func HandleUDPConnection(ctx context.Context, dialContext func(ctx context.Context, network string, addr string) (net.Conn, error), conn net.PacketConn, guestAddr string) {
	proxy, err := forwarder.NewUDPProxy(conn, func() (net.Conn, error) {
		return dialContext(ctx, "udp", guestAddr)
	})
	if err != nil {
		logrus.WithError(err).Error("error in udp tunnel proxy")
		return
	}

	defer func() {
		err := proxy.Close()
		if err != nil {
			logrus.WithError(err).Error("error in closing udp tunnel proxy")
		}
	}()
	proxy.Run()
	logrus.Debugf("udp proxy for guestAddr: %s closed", guestAddr)
}

func DialContextToGRPCTunnel(client *guestagentclient.GuestAgentClient) func(ctx context.Context, network, addr string) (net.Conn, error) {
	// gvisor-tap-vsock's UDPProxy demultiplexes client connections internally based on their source address.
	// It calls this dialer function only when it receives a datagram from a new, unrecognized client.
	// For each new client, we must return a new net.Conn, which in our case is a new gRPC stream.
	// The atomic counter ensures that each stream has a unique ID to distinguish them on the server side.
	var connectionCounter atomic.Uint32
	return func(_ context.Context, network, addr string) (net.Conn, error) {
		// Passed context.Context is used for timeout on initiate connection, not for the lifetime of the connection.
		// Use a dedicated context so CloseRead can unblock a goroutine waiting in stream.Recv().
		streamCtx, cancel := context.WithCancel(context.Background())
		stream, err := client.Tunnel(streamCtx)
		if err != nil {
			cancel()
			return nil, fmt.Errorf("could not open tunnel for addr: %s error:%w", addr, err)
		}
		// Handshake message to start tunnel
		id := fmt.Sprintf("%s-%s-%d", network, addr, connectionCounter.Add(1))
		if err := stream.Send(&api.TunnelMessage{Id: id, Protocol: network, GuestAddr: addr}); err != nil {
			cancel()
			return nil, fmt.Errorf("could not start tunnel for id: %s addr: %s error:%w", id, addr, err)
		}
		rw := &GrpcClientRW{stream: stream, cancel: cancel, id: id, addr: addr, protocol: network}
		return rw, nil
	}
}

type GrpcClientRW struct {
	id   string
	addr string

	protocol string
	stream   api.GuestService_TunnelClient
	cancel   context.CancelFunc
}

var _ net.Conn = (*GrpcClientRW)(nil)

func (g *GrpcClientRW) Write(p []byte) (n int, err error) {
	err = g.stream.Send(&api.TunnelMessage{
		Id:        g.id,
		GuestAddr: g.addr,
		Data:      p,
		Protocol:  g.protocol,
	})
	if err != nil {
		return 0, err
	}
	return len(p), nil
}

func (g *GrpcClientRW) Read(p []byte) (n int, err error) {
	in, err := g.stream.Recv()
	if err != nil {
		return 0, err
	}
	copy(p, in.Data)
	return len(in.Data), nil
}

func (g *GrpcClientRW) Close() error {
	logrus.Debugf("closing GrpcClientRW for id: %s", g.id)
	err := g.stream.CloseSend()
	g.cancel()
	return err
}

func (g *GrpcClientRW) CloseRead() error {
	logrus.Debugf("closing read GrpcClientRW for id: %s", g.id)
	g.cancel()
	return nil
}

func (g *GrpcClientRW) CloseWrite() error {
	logrus.Debugf("closing write GrpcClientRW for id: %s", g.id)
	err := g.stream.CloseSend()
	if errors.Is(err, context.Canceled) {
		return nil
	}
	return err
}

func (g *GrpcClientRW) LocalAddr() net.Addr {
	return &net.UnixAddr{Name: "grpc", Net: "unixpacket"}
}

func (g *GrpcClientRW) RemoteAddr() net.Addr {
	return &net.UnixAddr{Name: "grpc", Net: "unixpacket"}
}

func (g *GrpcClientRW) SetDeadline(_ time.Time) error {
	return nil
}

func (g *GrpcClientRW) SetReadDeadline(_ time.Time) error {
	return nil
}

func (g *GrpcClientRW) SetWriteDeadline(_ time.Time) error {
	return nil
}
