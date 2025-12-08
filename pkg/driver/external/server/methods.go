// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package server

import (
	"context"
	"encoding/json"
	"maps"
	"net"
	"path/filepath"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/lima-vm/lima/v2/pkg/bicopy"
	pb "github.com/lima-vm/lima/v2/pkg/driver/external"
	"github.com/lima-vm/lima/v2/pkg/limatype"
	"github.com/lima-vm/lima/v2/pkg/limatype/filenames"
)

func (s *DriverServer) Start(_ *emptypb.Empty, stream pb.Driver_StartServer) error {
	s.logger.Debug("Received Start request")
	errChan, err := s.driver.Start(stream.Context())
	if err != nil {
		s.logger.WithError(err).Error("Start failed")
		if sendErr := stream.Send(&pb.StartResponse{Success: false, Error: err.Error()}); sendErr != nil {
			s.logger.WithError(sendErr).Error("Failed to send error response")
			return status.Errorf(codes.Internal, "failed to send error response: %v", sendErr)
		}
		return status.Errorf(codes.Internal, "failed to start driver: %v", err)
	}

	// First send a success response upon receiving the errChan to unblock the client
	// and start receiving errors (if any).
	if err := stream.Send(&pb.StartResponse{Success: true}); err != nil {
		s.logger.WithError(err).Error("Failed to send success response")
		return status.Errorf(codes.Internal, "failed to send success response: %v", err)
	}

	for {
		select {
		case err, ok := <-errChan:
			if !ok {
				s.logger.Debug("Start error channel closed")
				if err := stream.Send(&pb.StartResponse{Success: true}); err != nil {
					s.logger.WithError(err).Error("Failed to send success response")
					return status.Errorf(codes.Internal, "failed to send success response: %v", err)
				}
				return nil
			}
			if err != nil {
				s.logger.WithError(err).Error("Error during Start")
				if err := stream.Send(&pb.StartResponse{Error: err.Error(), Success: false}); err != nil {
					s.logger.WithError(err).Error("Failed to send response")
					return status.Errorf(codes.Internal, "failed to send error response: %v", err)
				}
			}
		case <-stream.Context().Done():
			s.logger.Debug("Stream context done, stopping Start")
			return nil
		}
	}
}

func (s *DriverServer) Configure(_ context.Context, req *pb.SetConfigRequest) (*emptypb.Empty, error) {
	s.logger.Debugf("Received SetConfig request")
	var inst limatype.Instance

	if err := inst.UnmarshalJSON(req.InstanceConfigJson); err != nil {
		s.logger.WithError(err).Error("Failed to unmarshal InstanceConfigJson")
		return &emptypb.Empty{}, err
	}

	_ = s.driver.Configure(&inst)

	return &emptypb.Empty{}, nil
}

func (s *DriverServer) GuestAgentConn(ctx context.Context, _ *emptypb.Empty) (*emptypb.Empty, error) {
	s.logger.Debug("Received GuestAgentConn request")
	conn, connType, err := s.driver.GuestAgentConn(ctx)
	if err != nil {
		s.logger.WithError(err).Error("GuestAgentConn failed")
		return nil, err
	}

	if connType != "unix" {
		proxySocketPath := filepath.Join(s.driver.Info().InstanceDir, filenames.GuestAgentSock)

		var lc net.ListenConfig
		listener, err := lc.Listen(ctx, "unix", proxySocketPath)
		if err != nil {
			s.logger.WithError(err).Error("Failed to create proxy socket")
			return nil, err
		}

		go func() {
			defer listener.Close()
			defer conn.Close()

			proxyConn, err := listener.Accept()
			if err != nil {
				s.logger.WithError(err).Error("Failed to accept proxy connection")
				return
			}

			bicopy.Bicopy(conn, proxyConn, nil)
		}()
	}

	return &emptypb.Empty{}, nil
}

func (s *DriverServer) Info(_ context.Context, _ *emptypb.Empty) (*pb.InfoResponse, error) {
	s.logger.Debug("Received GetInfo request")
	info := s.driver.Info()

	infoJSON, err := json.Marshal(info)
	if err != nil {
		s.logger.WithError(err).Error("Failed to marshal driver info")
		return nil, status.Errorf(codes.Internal, "failed to marshal driver info: %v", err)
	}

	return &pb.InfoResponse{
		InfoJson: infoJSON,
	}, nil
}

func (s *DriverServer) Validate(ctx context.Context, empty *emptypb.Empty) (*emptypb.Empty, error) {
	s.logger.Debugf("Received Validate request")
	err := s.driver.Validate(ctx)
	if err != nil {
		s.logger.WithError(err).Error("Validation failed")
		return empty, err
	}
	s.logger.Debug("Validation succeeded")
	return empty, nil
}

func (s *DriverServer) Create(ctx context.Context, empty *emptypb.Empty) (*emptypb.Empty, error) {
	s.logger.Debug("Received Initialize request")
	err := s.driver.Create(ctx)
	if err != nil {
		s.logger.WithError(err).Error("Initialization failed")
		return empty, err
	}
	s.logger.Debug("Initialization succeeded")
	return empty, nil
}

func (s *DriverServer) CreateDisk(ctx context.Context, empty *emptypb.Empty) (*emptypb.Empty, error) {
	s.logger.Debug("Received CreateDisk request")
	err := s.driver.CreateDisk(ctx)
	if err != nil {
		s.logger.WithError(err).Error("CreateDisk failed")
		return empty, err
	}
	s.logger.Debug("CreateDisk succeeded")
	return empty, nil
}

func (s *DriverServer) Stop(ctx context.Context, empty *emptypb.Empty) (*emptypb.Empty, error) {
	s.logger.Debug("Received Stop request")
	err := s.driver.Stop(ctx)
	if err != nil {
		s.logger.WithError(err).Error("Stop failed")
		return empty, err
	}
	s.logger.Debug("Stop succeeded")
	return empty, nil
}

func (s *DriverServer) RunGUI(_ context.Context, empty *emptypb.Empty) (*emptypb.Empty, error) {
	s.logger.Debug("Received RunGUI request")
	err := s.driver.RunGUI()
	if err != nil {
		s.logger.WithError(err).Error("RunGUI failed")
		return empty, err
	}
	s.logger.Debug("RunGUI succeeded")
	return empty, nil
}

func (s *DriverServer) Delete(ctx context.Context, empty *emptypb.Empty) (*emptypb.Empty, error) {
	s.logger.Debug("Received Delete request")
	err := s.driver.Delete(ctx)
	if err != nil {
		s.logger.WithError(err).Error("Delete failed")
		return empty, err
	}
	s.logger.Debug("Delete succeeded")
	return empty, nil
}

func (s *DriverServer) BootScripts(_ context.Context, _ *emptypb.Empty) (*pb.BootScriptsResponse, error) {
	s.logger.Debug("Received BootScripts request")
	scripts, err := s.driver.BootScripts()
	if err != nil {
		s.logger.WithError(err).Error("BootScripts failed")
		return nil, err
	}

	resp := &pb.BootScriptsResponse{
		Scripts: make(map[string][]byte),
	}

	maps.Copy(resp.Scripts, scripts)

	s.logger.Debugf("BootScripts succeeded with %d scripts", len(resp.Scripts))
	return resp, nil
}

func (s *DriverServer) SSHAddress(ctx context.Context, _ *emptypb.Empty) (*pb.SSHAddressResponse, error) {
	s.logger.Debug("Received SSHAddress request")
	address, err := s.driver.SSHAddress(ctx)
	if err != nil {
		s.logger.WithError(err).Error("SSHAddress failed")
		return nil, err
	}

	s.logger.Debugf("SSHAddress succeeded with address: %s", address)
	return &pb.SSHAddressResponse{Address: address}, nil
}

func (s *DriverServer) ChangeDisplayPassword(ctx context.Context, req *pb.ChangeDisplayPasswordRequest) (*emptypb.Empty, error) {
	s.logger.Debug("Received ChangeDisplayPassword request")
	err := s.driver.ChangeDisplayPassword(ctx, req.Password)
	if err != nil {
		s.logger.WithError(err).Error("ChangeDisplayPassword failed")
		return &emptypb.Empty{}, err
	}
	s.logger.Debug("ChangeDisplayPassword succeeded")
	return &emptypb.Empty{}, nil
}

func (s *DriverServer) GetDisplayConnection(ctx context.Context, _ *emptypb.Empty) (*pb.GetDisplayConnectionResponse, error) {
	s.logger.Debug("Received GetDisplayConnection request")
	conn, err := s.driver.DisplayConnection(ctx)
	if err != nil {
		s.logger.WithError(err).Error("GetDisplayConnection failed")
		return nil, err
	}
	s.logger.Debug("GetDisplayConnection succeeded")
	return &pb.GetDisplayConnectionResponse{Connection: conn}, nil
}

func (s *DriverServer) CreateSnapshot(ctx context.Context, req *pb.CreateSnapshotRequest) (*emptypb.Empty, error) {
	s.logger.Debugf("Received CreateSnapshot request with tag: %s", req.Tag)
	err := s.driver.CreateSnapshot(ctx, req.Tag)
	if err != nil {
		s.logger.WithError(err).Error("CreateSnapshot failed")
		return &emptypb.Empty{}, err
	}
	s.logger.Debug("CreateSnapshot succeeded")
	return &emptypb.Empty{}, nil
}

func (s *DriverServer) ApplySnapshot(ctx context.Context, req *pb.ApplySnapshotRequest) (*emptypb.Empty, error) {
	s.logger.Debugf("Received ApplySnapshot request with tag: %s", req.Tag)
	err := s.driver.ApplySnapshot(ctx, req.Tag)
	if err != nil {
		s.logger.WithError(err).Error("ApplySnapshot failed")
		return &emptypb.Empty{}, err
	}
	s.logger.Debug("ApplySnapshot succeeded")
	return &emptypb.Empty{}, nil
}

func (s *DriverServer) DeleteSnapshot(ctx context.Context, req *pb.DeleteSnapshotRequest) (*emptypb.Empty, error) {
	s.logger.Debugf("Received DeleteSnapshot request with tag: %s", req.Tag)
	err := s.driver.DeleteSnapshot(ctx, req.Tag)
	if err != nil {
		s.logger.WithError(err).Error("DeleteSnapshot failed")
		return &emptypb.Empty{}, err
	}
	s.logger.Debug("DeleteSnapshot succeeded")
	return &emptypb.Empty{}, nil
}

func (s *DriverServer) ListSnapshots(ctx context.Context, _ *emptypb.Empty) (*pb.ListSnapshotsResponse, error) {
	s.logger.Debug("Received ListSnapshots request")
	snapshots, err := s.driver.ListSnapshots(ctx)
	if err != nil {
		s.logger.WithError(err).Error("ListSnapshots failed")
		return nil, err
	}
	s.logger.Debug("ListSnapshots succeeded")
	return &pb.ListSnapshotsResponse{Snapshots: snapshots}, nil
}

func (s *DriverServer) ForwardGuestAgent(_ context.Context, _ *emptypb.Empty) (*pb.ForwardGuestAgentResponse, error) {
	s.logger.Debug("Received ForwardGuestAgent request")
	return &pb.ForwardGuestAgentResponse{ShouldForward: s.driver.ForwardGuestAgent()}, nil
}

func (s *DriverServer) AdditionalSetupForSSH(ctx context.Context, _ *emptypb.Empty) (*emptypb.Empty, error) {
	s.logger.Debug("Received AdditionalSetupForSSH request")
	err := s.driver.AdditionalSetupForSSH(ctx)
	if err != nil {
		s.logger.WithError(err).Error("AdditionalSetupForSSH failed")
		return &emptypb.Empty{}, err
	}
	s.logger.Debug("AdditionalSetupForSSH succeeded")
	return &emptypb.Empty{}, nil
}
