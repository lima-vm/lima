// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package portfwd

import (
	"context"
	"fmt"
	"net"
	"sync/atomic"
	"time"

	"github.com/containers/gvisor-tap-vsock/pkg/services/forwarder"
	"github.com/containers/gvisor-tap-vsock/pkg/tcpproxy"
	"github.com/sirupsen/logrus"

	"github.com/lima-vm/lima/v2/pkg/guestagent/api"
	guestagentclient "github.com/lima-vm/lima/v2/pkg/guestagent/api/client"
)

func HandleTCPConnection(ctx context.Context, client *guestagentclient.GuestAgentClient, conn net.Conn, guestAddr string) {
	id := fmt.Sprintf("tcp-%s-%s", conn.LocalAddr().String(), conn.RemoteAddr().String())

	stream, err := client.Tunnel(ctx)
	if err != nil {
		logrus.Errorf("could not open tcp tunnel for id: %s error:%v", id, err)
		return
	}

	// Handshake message to start tunnel
	if err := stream.Send(&api.TunnelMessage{Id: id, Protocol: "tcp", GuestAddr: guestAddr}); err != nil {
		logrus.Errorf("could not start tcp tunnel for id: %s error:%v", id, err)
		return
	}

	rw := &GrpcClientRW{stream: stream, id: id, addr: guestAddr, protocol: "tcp"}
	proxy := tcpproxy.DialProxy{DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
		return conn, nil
	}}
	proxy.HandleConn(rw)
}

func HandleUDPConnection(ctx context.Context, client *guestagentclient.GuestAgentClient, conn net.PacketConn, guestAddr string) {
	var udpConnectionCounter atomic.Uint32
	initialID := fmt.Sprintf("udp-%s", conn.LocalAddr().String())

	// gvisor-tap-vsock's UDPProxy demultiplexes client connections internally based on their source address.
	// It calls this dialer function only when it receives a datagram from a new, unrecognized client.
	// For each new client, we must return a new net.Conn, which in our case is a new gRPC stream.
	// The atomic counter ensures that each stream has a unique ID to distinguish them on the server side.
	proxy, err := forwarder.NewUDPProxy(conn, func() (net.Conn, error) {
		id := fmt.Sprintf("%s-%d", initialID, udpConnectionCounter.Add(1))
		stream, err := client.Tunnel(ctx)
		if err != nil {
			return nil, fmt.Errorf("could not open udp tunnel for id: %s error:%w", id, err)
		}
		// Handshake message to start tunnel
		if err := stream.Send(&api.TunnelMessage{Id: id, Protocol: "udp", GuestAddr: guestAddr}); err != nil {
			return nil, fmt.Errorf("could not start udp tunnel for id: %s error:%w", id, err)
		}
		rw := &GrpcClientRW{stream: stream, id: id, addr: guestAddr, protocol: "udp"}
		return rw, nil
	})
	if err != nil {
		logrus.Errorf("error in udp tunnel proxy for id: %s error:%v", initialID, err)
		return
	}

	defer func() {
		err := proxy.Close()
		if err != nil {
			logrus.Errorf("error in closing udp tunnel proxy for id: %s error:%v", initialID, err)
		}
	}()
	proxy.Run()
}

type GrpcClientRW struct {
	id   string
	addr string

	protocol string
	stream   api.GuestService_TunnelClient
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
	return g.stream.CloseSend()
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
