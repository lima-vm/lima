/*
Copyright The Lima Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

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
