// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package server

import (
	"context"
	"net"

	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/lima-vm/lima/v2/pkg/guestagent"
	"github.com/lima-vm/lima/v2/pkg/guestagent/api"
	"github.com/lima-vm/lima/v2/pkg/portfwdserver"
)

func StartServer(ctx context.Context, lis net.Listener, guest *GuestServer) error {
	server := grpc.NewServer(
		grpc.InitialWindowSize(512<<20),
		grpc.InitialConnWindowSize(512<<20),
		grpc.ReadBufferSize(8<<20),
		grpc.WriteBufferSize(8<<20),
		grpc.MaxConcurrentStreams(2048),
		grpc.KeepaliveParams(keepalive.ServerParameters{Time: 0, Timeout: 0, MaxConnectionIdle: 0}),
	)
	api.RegisterGuestServiceServer(server, guest)
	go func() {
		<-ctx.Done()
		logrus.Debug("Stopping the gRPC server")
		server.GracefulStop()
		logrus.Debug("Closing the listener used by the gRPC server")
		lis.Close()
	}()
	err := server.Serve(lis)
	// grpc.Server.Serve() expects to return a non-nil error caused by lis.Accept()
	if opErr, ok := err.(*net.OpError); ok && opErr.Err.Error() == "use of closed network connection" {
		return nil
	}
	return err
}

type GuestServer struct {
	api.UnimplementedGuestServiceServer
	Agent   guestagent.Agent
	TunnelS *portfwdserver.TunnelServer
}

func (s *GuestServer) GetInfo(ctx context.Context, _ *emptypb.Empty) (*api.Info, error) {
	return s.Agent.Info(ctx)
}

func (s *GuestServer) GetEvents(_ *emptypb.Empty, stream api.GuestService_GetEventsServer) error {
	responses := make(chan *api.Event)
	// expects Events() to close the channel when stream.Context() is done or ticker stops
	go s.Agent.Events(stream.Context(), responses)
	for response := range responses {
		err := stream.Send(response)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *GuestServer) PostInotify(server api.GuestService_PostInotifyServer) error {
	for {
		recv, err := server.Recv()
		if err != nil {
			return err
		}
		s.Agent.HandleInotify(recv)
	}
}

func (s *GuestServer) Tunnel(stream api.GuestService_TunnelServer) error {
	return s.TunnelS.Start(stream)
}
