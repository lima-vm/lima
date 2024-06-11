package client

import (
	"context"
	"net"

	"github.com/lima-vm/lima/pkg/guestagent/api"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
)

type GuestAgentClient struct {
	cli api.GuestServiceClient
}

func NewGuestAgentClient(dialFn func(ctx context.Context) (net.Conn, error)) (*GuestAgentClient, error) {
	opts := []grpc.DialOption{
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return dialFn(ctx)
		}),
		grpc.WithTransportCredentials(NewCredentials()),
	}

	clientConn, err := grpc.Dial("", opts...) //nolint:staticcheck // SA1019: grpc.Dial is deprecated: use NewClient instead.  Will be supported throughout 1.x
	if err != nil {
		return nil, err
	}
	client := api.NewGuestServiceClient(clientConn)
	return &GuestAgentClient{
		cli: client,
	}, nil
}

func (c *GuestAgentClient) Info(ctx context.Context) (*api.Info, error) {
	return c.cli.GetInfo(ctx, &emptypb.Empty{})
}

func (c *GuestAgentClient) Events(ctx context.Context, eventCb func(response *api.Event)) error {
	events, err := c.cli.GetEvents(ctx, &emptypb.Empty{})
	if err != nil {
		return err
	}

	for {
		recv, err := events.Recv()
		if err != nil {
			return err
		}
		eventCb(recv)
	}
}

func (c *GuestAgentClient) Inotify(ctx context.Context) (api.GuestService_PostInotifyClient, error) {
	inotify, err := c.cli.PostInotify(ctx)
	if err != nil {
		return nil, err
	}
	return inotify, nil
}
