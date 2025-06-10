// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	pb "github.com/lima-vm/lima/pkg/driver/external"
	"github.com/lima-vm/lima/pkg/store"
)

func (s *DriverServer) Start(empty *emptypb.Empty, stream pb.Driver_StartServer) error {
	s.logger.Debug("Received Start request")
	errChan, err := s.driver.Start(stream.Context())
	if err != nil {
		s.logger.Errorf("Start failed: %v", err)
		return nil
	}

	for {
		select {
		case err, ok := <-errChan:
			if !ok {
				s.logger.Debug("Start error channel closed")
				if err := stream.Send(&pb.StartResponse{Success: true}); err != nil {
					s.logger.Errorf("Failed to send success response: %v", err)
					return status.Errorf(codes.Internal, "failed to send success response: %v", err)
				}
				return nil
			}
			if err != nil {
				s.logger.Errorf("Error during Start: %v", err)
				if err := stream.Send(&pb.StartResponse{Error: err.Error(), Success: false}); err != nil {
					s.logger.Errorf("Failed to send error response: %v", err)
					return status.Errorf(codes.Internal, "failed to send error response: %v", err)
				}
			}
		case <-stream.Context().Done():
			s.logger.Debug("Stream context done, stopping Start")
			return nil
		}
	}
}

func (s *DriverServer) SetConfig(ctx context.Context, req *pb.SetConfigRequest) (*emptypb.Empty, error) {
	s.logger.Debugf("Received SetConfig request")
	var inst store.Instance

	if err := inst.UnmarshalJSON(req.InstanceConfigJson); err != nil {
		s.logger.Errorf("Failed to unmarshal InstanceConfigJson: %v", err)
		return &emptypb.Empty{}, err
	}

	s.driver.SetConfig(&inst, int(req.SshLocalPort))

	return &emptypb.Empty{}, nil
}

func (s *DriverServer) GuestAgentConn(stream pb.Driver_GuestAgentConnServer) error {
	s.logger.Debug("Received GuestAgentConn request")
	conn, err := s.driver.GuestAgentConn(context.Background())
	if err != nil {
		s.logger.Errorf("GuestAgentConn failed: %v", err)
		return err
	}
	s.logger.Debug("GuestAgentConn succeeded")

	go func() {
		for {
			msg, err := stream.Recv()
			if err != nil {
				return
			}
			s.logger.Debugf("Received message from stream: %d bytes", len(msg.Data))
			if len(msg.Data) > 0 {
				_, err = conn.Write(msg.Data)
				if err != nil {
					s.logger.Errorf("Error writing to connection: %v", err)
					conn.Close()
					return
				}
			}
		}
	}()

	buf := make([]byte, 32*1<<10)
	for {
		n, err := conn.Read(buf)
		if err != nil {
			if errors.Is(err, io.EOF) {
				s.logger.Debugf("Connection closed by guest agent %v", err)
				return nil
			}
			return status.Errorf(codes.Internal, "error reading: %v", err)
		}
		s.logger.Debugf("Sending %d bytes to stream", n)

		msg := &pb.BytesMessage{Data: buf[:n]}
		if err := stream.Send(msg); err != nil {
			s.logger.Errorf("Failed to send message to stream: %v", err)
			return err
		}
	}
}

func (s *DriverServer) GetInfo(ctx context.Context, empty *emptypb.Empty) (*pb.InfoResponse, error) {
	s.logger.Debug("Received GetInfo request")
	info := s.driver.GetInfo()

	infoJson, err := json.Marshal(info)
	if err != nil {
		s.logger.Errorf("Failed to marshal driver info: %v", err)
		return nil, status.Errorf(codes.Internal, "failed to marshal driver info: %v", err)
	}

	return &pb.InfoResponse{
		InfoJson: infoJson,
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
