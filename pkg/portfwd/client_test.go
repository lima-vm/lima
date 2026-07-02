// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package portfwd

import (
	"bytes"
	"errors"
	"io"
	"net"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/test/bufconn"
	"gotest.tools/v3/assert"

	"github.com/lima-vm/lima/v2/pkg/guestagent/api"
	guestagentclient "github.com/lima-vm/lima/v2/pkg/guestagent/api/client"
)

func TestDialContextToGRPCTunnelClosesStreamFromTCPProxy(t *testing.T) {
	tunnelDone := make(chan error, 1)
	client := newTestGuestAgentClient(t, &testGuestService{tunnelDone: tunnelDone})
	defer client.Close()

	clientConn, proxyConn := net.Pipe()
	proxyDone := make(chan struct{})
	go func() {
		HandleTCPConnection(t.Context(), DialContextToGRPCTunnel(client), proxyConn, "127.0.0.1:80")
		close(proxyDone)
	}()

	assert.NilError(t, clientConn.Close())

	select {
	case err := <-tunnelDone:
		assert.NilError(t, err)
	case <-time.After(time.Second):
		assert.Assert(t, false, "timed out waiting for the tunnel stream to close")
	}

	select {
	case <-proxyDone:
	case <-time.After(time.Second):
		assert.Assert(t, false, "timed out waiting for TCP proxy handling to finish")
	}
}

// recvOnlyTunnelClient implements only Recv of the bidi stream; the other
// methods are never called by GrpcClientRW.Read.
type recvOnlyTunnelClient struct {
	api.GuestService_TunnelClient
	msgs []*api.TunnelMessage
}

func (s *recvOnlyTunnelClient) Recv() (*api.TunnelMessage, error) {
	if len(s.msgs) == 0 {
		return nil, io.EOF
	}
	msg := s.msgs[0]
	s.msgs = s.msgs[1:]
	return msg, nil
}

func TestGrpcClientRWReadShortBuffer(t *testing.T) {
	payload := bytes.Repeat([]byte("x"), 100)
	rw := &GrpcClientRW{stream: &recvOnlyTunnelClient{msgs: []*api.TunnelMessage{{Data: payload}}}}

	var got []byte
	buf := make([]byte, 10)
	for len(got) < len(payload) {
		n, err := rw.Read(buf)
		assert.NilError(t, err)
		assert.Assert(t, n <= len(buf), "Read returned %d, larger than the %d-byte buffer", n, len(buf))
		got = append(got, buf[:n]...)
	}
	assert.DeepEqual(t, got, payload)
}

func newTestGuestAgentClient(t *testing.T, guestService api.GuestServiceServer) *guestagentclient.GuestAgentClient {
	t.Helper()

	listener := bufconn.Listen(1024 * 1024)
	server := grpc.NewServer(grpc.Creds(guestagentclient.NewCredentials()))
	api.RegisterGuestServiceServer(server, guestService)
	go func() {
		_ = server.Serve(listener)
	}()
	t.Cleanup(func() {
		server.Stop()
		_ = listener.Close()
	})

	client, err := guestagentclient.NewGuestAgentClient(listener.DialContext)
	assert.NilError(t, err)
	return client
}

type testGuestService struct {
	api.UnimplementedGuestServiceServer

	tunnelDone chan<- error
}

func (s *testGuestService) Tunnel(stream api.GuestService_TunnelServer) error {
	if _, err := stream.Recv(); err != nil {
		s.tunnelDone <- err
		return err
	}
	for {
		_, err := stream.Recv()
		if err == nil {
			continue
		}
		if errors.Is(err, io.EOF) {
			s.tunnelDone <- nil
			return nil
		}
		s.tunnelDone <- err
		return err
	}
}
