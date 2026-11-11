// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package server

import (
	"context"
	"encoding/json"
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/lima-vm/lima/v2/pkg/driver"
	pb "github.com/lima-vm/lima/v2/pkg/driver/external"
)

// usbStatus returns UNIMPLEMENTED for driver.ErrDriverNotUSBPassthrough.
func usbStatus(err error) error {
	if errors.Is(err, driver.ErrDriverNotUSBPassthrough) {
		return status.Error(codes.Unimplemented, err.Error())
	}
	return err
}

func (s *DriverServer) usbPassthrough() (driver.USBPassthrough, error) {
	p, ok := s.driver.(driver.USBPassthrough)
	if !ok {
		return nil, status.Error(codes.Unimplemented, driver.ErrDriverNotUSBPassthrough.Error())
	}
	return p, nil
}

func (s *DriverServer) ListUSBDevices(ctx context.Context, _ *emptypb.Empty) (*pb.ListUSBDevicesResponse, error) {
	s.logger.Debug("Received ListUSBDevices request")
	p, err := s.usbPassthrough()
	if err != nil {
		return nil, err
	}
	devs, err := p.ListUSBDevices(ctx)
	if err != nil {
		s.logger.WithError(err).Error("ListUSBDevices failed")
		return nil, usbStatus(err)
	}
	devsJSON, err := json.Marshal(devs)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to marshal USB devices: %v", err)
	}
	s.logger.Debug("ListUSBDevices succeeded")
	return &pb.ListUSBDevicesResponse{DevicesJson: devsJSON}, nil
}

func (s *DriverServer) AttachUSBDevice(ctx context.Context, req *pb.USBDeviceRequest) (*emptypb.Empty, error) {
	s.logger.Debugf("Received AttachUSBDevice request for %#x", req.RegistryId)
	p, err := s.usbPassthrough()
	if err != nil {
		return nil, err
	}
	if err := p.AttachUSBDevice(ctx, req.RegistryId); err != nil {
		s.logger.WithError(err).Error("AttachUSBDevice failed")
		return nil, usbStatus(err)
	}
	s.logger.Debug("AttachUSBDevice succeeded")
	return &emptypb.Empty{}, nil
}

func (s *DriverServer) DetachUSBDevice(ctx context.Context, req *pb.USBDeviceRequest) (*emptypb.Empty, error) {
	s.logger.Debugf("Received DetachUSBDevice request for %#x", req.RegistryId)
	p, err := s.usbPassthrough()
	if err != nil {
		return nil, err
	}
	if err := p.DetachUSBDevice(ctx, req.RegistryId); err != nil {
		s.logger.WithError(err).Error("DetachUSBDevice failed")
		return nil, usbStatus(err)
	}
	s.logger.Debug("DetachUSBDevice succeeded")
	return &emptypb.Empty{}, nil
}
