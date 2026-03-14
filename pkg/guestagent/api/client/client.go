// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"context"
	"math"
	"net"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/resolver"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/lima-vm/lima/v2/pkg/guestagent/api"
)

type GuestAgentClient struct {
	cli api.GuestServiceClient
}

func NewGuestAgentClient(dialFn func(ctx context.Context) (net.Conn, error)) (*GuestAgentClient, error) {
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(NewCredentials()),
		grpc.WithInitialWindowSize(512 << 20),
		grpc.WithInitialConnWindowSize(512 << 20),
		grpc.WithReadBufferSize(8 << 20),
		grpc.WithWriteBufferSize(8 << 20),
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(math.MaxInt32),
			grpc.MaxCallSendMsgSize(math.MaxInt32),
		),
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return dialFn(ctx)
		}),
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

func (c *GuestAgentClient) SyncTime(ctx context.Context, hostTime time.Time) (*api.TimeSyncResponse, error) {
	req := &api.TimeSyncRequest{
		HostTime: timestamppb.New(hostTime),
	}
	return c.cli.SyncTime(ctx, req)
}
