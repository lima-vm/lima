// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/lima-vm/lima/v2/pkg/driver"
	pb "github.com/lima-vm/lima/v2/pkg/driver/external"
)

var _ driver.USBPassthrough = (*DriverClient)(nil)

// usbError restores driver.ErrDriverNotUSBPassthrough from UNIMPLEMENTED, which
// the host agent API maps to 501.
func (d *DriverClient) usbError(err error, msg string) error {
	s := status.Convert(err)
	if s.Code() == codes.Unimplemented {
		d.logger.WithError(err).Debug(msg)
		reason := strings.TrimPrefix(s.Message(), driver.ErrDriverNotUSBPassthrough.Error())
		return fmt.Errorf("%w%s", driver.ErrDriverNotUSBPassthrough, reason)
	}
	d.logger.WithError(err).Error(msg)
	return errors.New(s.Message())
}

func (d *DriverClient) ListUSBDevices(ctx context.Context) ([]driver.USBDevice, error) {
	d.logger.Debug("Listing USB devices")

	resp, err := d.DriverSvc.ListUSBDevices(ctx, &emptypb.Empty{})
	if err != nil {
		return nil, d.usbError(err, "Failed to list USB devices")
	}
	var devs []driver.USBDevice
	if err := json.Unmarshal(resp.DevicesJson, &devs); err != nil {
		return nil, fmt.Errorf("failed to unmarshal USB devices: %w", err)
	}
	d.logger.Debugf("USB devices listed successfully: %+v", devs)
	return devs, nil
}

func (d *DriverClient) AttachUSBDevice(ctx context.Context, registryID uint64) error {
	d.logger.Debugf("Attaching USB device %#x", registryID)

	_, err := d.DriverSvc.AttachUSBDevice(ctx, &pb.USBDeviceRequest{RegistryId: registryID})
	if err != nil {
		return d.usbError(err, "Failed to attach USB device")
	}
	d.logger.Debugf("USB device %#x attached successfully", registryID)
	return nil
}

func (d *DriverClient) DetachUSBDevice(ctx context.Context, registryID uint64) error {
	d.logger.Debugf("Detaching USB device %#x", registryID)

	_, err := d.DriverSvc.DetachUSBDevice(ctx, &pb.USBDeviceRequest{RegistryId: registryID})
	if err != nil {
		return d.usbError(err, "Failed to detach USB device")
	}
	d.logger.Debugf("USB device %#x detached successfully", registryID)
	return nil
}
