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

package server

import (
	"context"
	"net"

	"github.com/lima-vm/lima/pkg/guestagent"
	"github.com/lima-vm/lima/pkg/guestagent/api"
	"github.com/lima-vm/lima/pkg/portfwdserver"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
)

func StartServer(lis net.Listener, guest *GuestServer) error {
	server := grpc.NewServer()
	api.RegisterGuestServiceServer(server, guest)
	return server.Serve(lis)
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
