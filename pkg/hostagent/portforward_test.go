// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package hostagent

import (
	"context"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/types/known/emptypb"
	"gotest.tools/v3/assert"

	"github.com/lima-vm/lima/v2/pkg/guestagent/api"
	guestagentclient "github.com/lima-vm/lima/v2/pkg/guestagent/api/client"
)

// invalidIPGuestService streams one port event whose IP is not an IP address.
// A compromised guest can report an arbitrary string there; this one carries
// ANSI escapes that would rewrite the operator's terminal if it reached
// PortForwardEvent.GuestAddr.
type invalidIPGuestService struct {
	api.UnimplementedGuestServiceServer
}

func (*invalidIPGuestService) GetInfo(context.Context, *emptypb.Empty) (*api.Info, error) {
	return &api.Info{}, nil
}

func (*invalidIPGuestService) GetEvents(_ *emptypb.Empty, stream api.GuestService_GetEventsServer) error {
	return stream.Send(&api.Event{
		AddedLocalPorts: []*api.IPPort{{Ip: "1.2.3.4\x1b[2Kspoofed\x07", Port: 8080, Protocol: "tcp"}},
	})
}

func TestProcessGuestAgentEventsRejectsInvalidIP(t *testing.T) {
	listener := bufconn.Listen(1024 * 1024)
	server := grpc.NewServer(grpc.Creds(guestagentclient.NewCredentials()))
	api.RegisterGuestServiceServer(server, &invalidIPGuestService{})
	go func() {
		_ = server.Serve(listener)
	}()
	t.Cleanup(func() {
		server.Stop()
		_ = listener.Close()
	})

	client, err := guestagentclient.NewGuestAgentClient(listener.DialContext)
	assert.NilError(t, err)
	t.Cleanup(func() { _ = client.Close() })

	a := &HostAgent{guestAgentAliveCh: make(chan struct{})}
	err = a.processGuestAgentEvents(t.Context(), client)
	assert.ErrorContains(t, err, "invalid IP")
}
