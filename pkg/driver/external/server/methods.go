// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package server

import (
	"context"

	pb "github.com/lima-vm/lima/pkg/driver/external"
	"google.golang.org/protobuf/types/known/emptypb"
)

// TODO: Add more 3 functions Start, SetConfig & GuestAgentConn

func (s *DriverServer) Name(ctx context.Context, empty *emptypb.Empty) (*pb.NameResponse, error) {
	s.logger.Debug("Received Name request")
	return &pb.NameResponse{Name: s.driver.Name()}, nil
}

func (s *DriverServer) GetVirtioPort(ctx context.Context, empty *emptypb.Empty) (*pb.GetVirtioPortResponse, error) {
	s.logger.Debug("Received GetVirtioPort request")
	return &pb.GetVirtioPortResponse{
		Port: s.driver.GetVirtioPort(),
	}, nil
}

func (s *DriverServer) GetVSockPort(ctx context.Context, empty *emptypb.Empty) (*pb.GetVSockPortResponse, error) {
	s.logger.Debug("Received GetVSockPort request")
	return &pb.GetVSockPortResponse{
		Port: int64(s.driver.GetVSockPort()),
	}, nil
}

func (s *DriverServer) Validate(ctx context.Context, empty *emptypb.Empty) (*emptypb.Empty, error) {
	s.logger.Debugf("Received Validate request")
	err := s.driver.Validate()
	if err != nil {
		s.logger.Errorf("Validation failed: %v", err)
		return empty, err
	}
	s.logger.Debug("Validation succeeded")
	return empty, nil
}

func (s *DriverServer) Initialize(ctx context.Context, empty *emptypb.Empty) (*emptypb.Empty, error) {
	s.logger.Debug("Received Initialize request")
	err := s.driver.Initialize(ctx)
	if err != nil {
		s.logger.Errorf("Initialization failed: %v", err)
		return empty, err
	}
	s.logger.Debug("Initialization succeeded")
	return empty, nil
}

func (s *DriverServer) CreateDisk(ctx context.Context, empty *emptypb.Empty) (*emptypb.Empty, error) {
	s.logger.Debug("Received CreateDisk request")
	err := s.driver.CreateDisk(ctx)
	if err != nil {
		s.logger.Errorf("CreateDisk failed: %v", err)
		return empty, err
	}
	s.logger.Debug("CreateDisk succeeded")
	return empty, nil
}

func (s *DriverServer) Stop(ctx context.Context, empty *emptypb.Empty) (*emptypb.Empty, error) {
	s.logger.Debug("Received Stop request")
	err := s.driver.Stop(ctx)
	if err != nil {
		s.logger.Errorf("Stop failed: %v", err)
		return empty, err
	}
	s.logger.Debug("Stop succeeded")
	return empty, nil
}

func (s *DriverServer) CanRunGUI(ctx context.Context, empty *emptypb.Empty) (*pb.CanRunGUIResponse, error) {
	s.logger.Debug("Received CanRunGUI request")
	return &pb.CanRunGUIResponse{CanRun: s.driver.CanRunGUI()}, nil
}

func (s *DriverServer) RunGUI(ctx context.Context, empty *emptypb.Empty) (*emptypb.Empty, error) {
	s.logger.Debug("Received RunGUI request")
	err := s.driver.RunGUI()
	if err != nil {
		s.logger.Errorf("RunGUI failed: %v", err)
		return empty, err
	}
	s.logger.Debug("RunGUI succeeded")
	return empty, nil
}

func (s *DriverServer) ChangeDisplayPassword(ctx context.Context, req *pb.ChangeDisplayPasswordRequest) (*emptypb.Empty, error) {
	s.logger.Debug("Received ChangeDisplayPassword request")
	err := s.driver.ChangeDisplayPassword(ctx, req.Password)
	if err != nil {
		s.logger.Errorf("ChangeDisplayPassword failed: %v", err)
		return &emptypb.Empty{}, err
	}
	s.logger.Debug("ChangeDisplayPassword succeeded")
	return &emptypb.Empty{}, nil
}

func (s *DriverServer) GetDisplayConnection(ctx context.Context, empty *emptypb.Empty) (*pb.GetDisplayConnectionResponse, error) {
	s.logger.Debug("Received GetDisplayConnection request")
	conn, err := s.driver.GetDisplayConnection(ctx)
	if err != nil {
		s.logger.Errorf("GetDisplayConnection failed: %v", err)
		return nil, err
	}
	s.logger.Debug("GetDisplayConnection succeeded")
	return &pb.GetDisplayConnectionResponse{Connection: conn}, nil
}

func (s *DriverServer) CreateSnapshot(ctx context.Context, req *pb.CreateSnapshotRequest) (*emptypb.Empty, error) {
	s.logger.Debugf("Received CreateSnapshot request with tag: %s", req.Tag)
	err := s.driver.CreateSnapshot(ctx, req.Tag)
	if err != nil {
		s.logger.Errorf("CreateSnapshot failed: %v", err)
		return &emptypb.Empty{}, err
	}
	s.logger.Debug("CreateSnapshot succeeded")
	return &emptypb.Empty{}, nil
}

func (s *DriverServer) ApplySnapshot(ctx context.Context, req *pb.ApplySnapshotRequest) (*emptypb.Empty, error) {
	s.logger.Debugf("Received ApplySnapshot request with tag: %s", req.Tag)
	err := s.driver.ApplySnapshot(ctx, req.Tag)
	if err != nil {
		s.logger.Errorf("ApplySnapshot failed: %v", err)
		return &emptypb.Empty{}, err
	}
	s.logger.Debug("ApplySnapshot succeeded")
	return &emptypb.Empty{}, nil
}

func (s *DriverServer) DeleteSnapshot(ctx context.Context, req *pb.DeleteSnapshotRequest) (*emptypb.Empty, error) {
	s.logger.Debugf("Received DeleteSnapshot request with tag: %s", req.Tag)
	err := s.driver.DeleteSnapshot(ctx, req.Tag)
	if err != nil {
		s.logger.Errorf("DeleteSnapshot failed: %v", err)
		return &emptypb.Empty{}, err
	}
	s.logger.Debug("DeleteSnapshot succeeded")
	return &emptypb.Empty{}, nil
}

func (s *DriverServer) ListSnapshots(ctx context.Context, empty *emptypb.Empty) (*pb.ListSnapshotsResponse, error) {
	s.logger.Debug("Received ListSnapshots request")
	snapshots, err := s.driver.ListSnapshots(ctx)
	if err != nil {
		s.logger.Errorf("ListSnapshots failed: %v", err)
		return nil, err
	}
	s.logger.Debug("ListSnapshots succeeded")
	return &pb.ListSnapshotsResponse{Snapshots: snapshots}, nil
}

func (s *DriverServer) Register(ctx context.Context, empty *emptypb.Empty) (*emptypb.Empty, error) {
	s.logger.Debug("Received Register request")
	err := s.driver.Register(ctx)
	if err != nil {
		s.logger.Errorf("Register failed: %v", err)
		return empty, err
	}
	s.logger.Debug("Register succeeded")
	return empty, nil
}

func (s *DriverServer) Unregister(ctx context.Context, empty *emptypb.Empty) (*emptypb.Empty, error) {
	s.logger.Debug("Received Unregister request")
	err := s.driver.Unregister(ctx)
	if err != nil {
		s.logger.Errorf("Unregister failed: %v", err)
		return empty, err
	}
	s.logger.Debug("Unregister succeeded")
	return empty, nil
}

func (s *DriverServer) ForwardGuestAgent(ctx context.Context, empty *emptypb.Empty) (*pb.ForwardGuestAgentResponse, error) {
	s.logger.Debug("Received ForwardGuestAgent request")
	return &pb.ForwardGuestAgentResponse{ShouldForward: s.driver.ForwardGuestAgent()}, nil
}
