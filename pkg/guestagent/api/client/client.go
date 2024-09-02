package client

import (
	"context"
	"math"
	"net"

	"github.com/lima-vm/lima/pkg/guestagent/api"
	"google.golang.org/grpc"
	"google.golang.org/grpc/resolver"
	"google.golang.org/protobuf/types/known/emptypb"
)

type GuestAgentClient struct {
	cli api.GuestServiceClient
}

func NewGuestAgentClient(dialFn func(ctx context.Context) (net.Conn, error)) (*GuestAgentClient, error) {
	opts := []grpc.DialOption{
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(math.MaxInt64),
			grpc.MaxCallSendMsgSize(math.MaxInt64),
		),
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return dialFn(ctx)
		}),
		grpc.WithTransportCredentials(NewCredentials()),
	}

	resolver.SetDefaultScheme("passthrough")
	clientConn, err := grpc.NewClient("", opts...)
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

func (c *GuestAgentClient) Tunnel(ctx context.Context) (api.GuestService_TunnelClient, error) {
	stream, err := c.cli.Tunnel(ctx)
	if err != nil {
		return nil, err
	}
	return stream, nil
}
