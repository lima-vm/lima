// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"time"

	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/lima-vm/lima/v2/pkg/driver"
	pb "github.com/lima-vm/lima/v2/pkg/driver/external"
	"github.com/lima-vm/lima/v2/pkg/limatype"
)

func (d *DriverClient) Validate(ctx context.Context) error {
	d.logger.Debug("Validating driver for the given config")

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	_, err := d.DriverSvc.Validate(ctx, &emptypb.Empty{})
	if err != nil {
		d.logger.Errorf("Validation failed: %v", err)
		return err
	}

	d.logger.Debug("Driver validated successfully")
	return nil
}

func (d *DriverClient) Initialize(ctx context.Context) error {
	d.logger.Debug("Initializing driver instance")

	_, err := d.DriverSvc.Initialize(ctx, &emptypb.Empty{})
	if err != nil {
		d.logger.Errorf("Initialization failed: %v", err)
		return err
	}

	d.logger.Debug("Driver instance initialized successfully")
	return nil
}

func (d *DriverClient) CreateDisk(ctx context.Context) error {
	d.logger.Debug("Creating disk for the instance")

	_, err := d.DriverSvc.CreateDisk(ctx, &emptypb.Empty{})
	if err != nil {
		d.logger.Errorf("Disk creation failed: %v", err)
		return err
	}

	d.logger.Debug("Disk created successfully")
	return nil
}

func (d *DriverClient) Start(ctx context.Context) (chan error, error) {
	d.logger.Debug("Starting driver instance")

	stream, err := d.DriverSvc.Start(ctx, &emptypb.Empty{})
	if err != nil {
		d.logger.Errorf("Failed to start driver instance: %v", err)
		return nil, err
	}

	errCh := make(chan error, 1)
	go func() {
		for {
			errorStream, err := stream.Recv()
			if err != nil {
				d.logger.Errorf("Error receiving response from driver: %v", err)
				return
			}
			d.logger.Debugf("Received response: %v", errorStream)
			if !errorStream.Success {
				errCh <- errors.New(errorStream.Error)
			} else {
				errCh <- nil
				return
			}
		}
	}()

	d.logger.Debug("Driver instance started successfully")
	return errCh, nil
}

func (d *DriverClient) Stop(ctx context.Context) error {
	d.logger.Debug("Stopping driver instance")

	_, err := d.DriverSvc.Stop(ctx, &emptypb.Empty{})
	if err != nil {
		d.logger.Errorf("Failed to stop driver instance: %v", err)
		return err
	}

	d.logger.Debug("Driver instance stopped successfully")
	return nil
}

func (d *DriverClient) RunGUI() error {
	d.logger.Debug("Running GUI for the driver instance")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := d.DriverSvc.RunGUI(ctx, &emptypb.Empty{})
	if err != nil {
		d.logger.Errorf("Failed to run GUI: %v", err)
		return err
	}

	d.logger.Debug("GUI started successfully")
	return nil
}

func (d *DriverClient) ChangeDisplayPassword(ctx context.Context, password string) error {
	d.logger.Debug("Changing display password for the driver instance")

	_, err := d.DriverSvc.ChangeDisplayPassword(ctx, &pb.ChangeDisplayPasswordRequest{
		Password: password,
	})
	if err != nil {
		d.logger.Errorf("Failed to change display password: %v", err)
		return err
	}

	d.logger.Debug("Display password changed successfully")
	return nil
}

func (d *DriverClient) DisplayConnection(ctx context.Context) (string, error) {
	d.logger.Debug("Getting display connection for the driver instance")

	resp, err := d.DriverSvc.GetDisplayConnection(ctx, &emptypb.Empty{})
	if err != nil {
		d.logger.Errorf("Failed to get display connection: %v", err)
		return "", err
	}

	d.logger.Debugf("Display connection retrieved: %s", resp.Connection)
	return resp.Connection, nil
}

func (d *DriverClient) CreateSnapshot(ctx context.Context, tag string) error {
	d.logger.Debugf("Creating snapshot with tag: %s", tag)

	_, err := d.DriverSvc.CreateSnapshot(ctx, &pb.CreateSnapshotRequest{
		Tag: tag,
	})
	if err != nil {
		d.logger.Errorf("Failed to create snapshot: %v", err)
		return err
	}

	d.logger.Debugf("Snapshot '%s' created successfully", tag)
	return nil
}

func (d *DriverClient) ApplySnapshot(ctx context.Context, tag string) error {
	d.logger.Debugf("Applying snapshot with tag: %s", tag)

	_, err := d.DriverSvc.ApplySnapshot(ctx, &pb.ApplySnapshotRequest{
		Tag: tag,
	})
	if err != nil {
		d.logger.Errorf("Failed to apply snapshot: %v", err)
		return err
	}

	d.logger.Debugf("Snapshot '%s' applied successfully", tag)
	return nil
}

func (d *DriverClient) DeleteSnapshot(ctx context.Context, tag string) error {
	d.logger.Debugf("Deleting snapshot with tag: %s", tag)

	_, err := d.DriverSvc.DeleteSnapshot(ctx, &pb.DeleteSnapshotRequest{
		Tag: tag,
	})
	if err != nil {
		d.logger.Errorf("Failed to delete snapshot: %v", err)
		return err
	}

	d.logger.Debugf("Snapshot '%s' deleted successfully", tag)
	return nil
}

func (d *DriverClient) ListSnapshots(ctx context.Context) (string, error) {
	d.logger.Debug("Listing snapshots")

	resp, err := d.DriverSvc.ListSnapshots(ctx, &emptypb.Empty{})
	if err != nil {
		d.logger.Errorf("Failed to list snapshots: %v", err)
		return "", err
	}

	d.logger.Debugf("Snapshots listed successfully: %s", resp.Snapshots)
	return resp.Snapshots, nil
}

func (d *DriverClient) Register(ctx context.Context) error {
	d.logger.Debug("Registering driver instance")

	_, err := d.DriverSvc.Register(ctx, &emptypb.Empty{})
	if err != nil {
		d.logger.Errorf("Failed to register driver instance: %v", err)
		return err
	}

	d.logger.Debug("Driver instance registered successfully")
	return nil
}

func (d *DriverClient) Unregister(ctx context.Context) error {
	d.logger.Debug("Unregistering driver instance")

	_, err := d.DriverSvc.Unregister(ctx, &emptypb.Empty{})
	if err != nil {
		d.logger.Errorf("Failed to unregister driver instance: %v", err)
		return err
	}

	d.logger.Debug("Driver instance unregistered successfully")
	return nil
}

func (d *DriverClient) ForwardGuestAgent() bool {
	d.logger.Debug("Checking if guest agent needs to be forwarded")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := d.DriverSvc.ForwardGuestAgent(ctx, &emptypb.Empty{})
	if err != nil {
		d.logger.Errorf("Failed to check guest agent forwarding: %v", err)
		return false
	}

	return resp.ShouldForward
}

func (d *DriverClient) GuestAgentConn(ctx context.Context) (net.Conn, string, error) {
	d.logger.Info("Getting guest agent connection")
	_, err := d.DriverSvc.GuestAgentConn(ctx, &emptypb.Empty{})
	if err != nil {
		d.logger.Errorf("Failed to get guest agent connection: %v", err)
		return nil, "", err
	}

	return nil, "", nil
}

func (d *DriverClient) Info() driver.Info {
	d.logger.Debug("Getting driver info")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := d.DriverSvc.GetInfo(ctx, &emptypb.Empty{})
	if err != nil {
		d.logger.Errorf("Failed to get driver info: %v", err)
		return driver.Info{}
	}

	var info driver.Info
	if err := json.Unmarshal(resp.InfoJson, &info); err != nil {
		d.logger.Errorf("Failed to unmarshal driver info: %v", err)
		return driver.Info{}
	}

	d.logger.Debugf("Driver info retrieved: %+v", info)
	return info
}

func (d *DriverClient) Configure(inst *store.Instance) *driver.ConfiguredDriver {
	d.logger.Debugf("Setting config for instance %s with SSH local port %d", inst.Name, inst.SSHLocalPort)

	instJSON, err := inst.MarshalJSON()
	if err != nil {
		d.logger.Errorf("Failed to marshal instance config: %v", err)
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err = d.DriverSvc.SetConfig(ctx, &pb.SetConfigRequest{
		InstanceConfigJson: instJSON,
	})
	if err != nil {
		d.logger.Errorf("Failed to set config: %v", err)
		return nil
	}

	d.logger.Debugf("Config set successfully for instance %s", inst.Name)
	return &driver.ConfiguredDriver{
		Driver: d,
	}
}

func (d *DriverClient) AcceptConfig(cfg *limatype.LimaYAML, filepath string) error {
	return errors.New("AcceptConfig not implemented in DriverClient")
}

func (d *DriverClient) FillConfig(cfg *limatype.LimaYAML, filepath string) error {
	return errors.New("AcceptConfig not implemented in DriverClient")
}
