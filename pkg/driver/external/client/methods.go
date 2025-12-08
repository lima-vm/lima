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
		d.logger.WithError(err).Error("Validation failed")
		return err
	}

	d.logger.Debug("Driver validated successfully")
	return nil
}

func (d *DriverClient) Create(ctx context.Context) error {
	d.logger.Debug("Initializing driver instance")

	_, err := d.DriverSvc.Create(ctx, &emptypb.Empty{})
	if err != nil {
		d.logger.WithError(err).Error("Initialization failed")
		return err
	}

	d.logger.Debug("Driver instance initialized successfully")
	return nil
}

func (d *DriverClient) CreateDisk(ctx context.Context) error {
	d.logger.Debug("Creating disk for the instance")

	_, err := d.DriverSvc.CreateDisk(ctx, &emptypb.Empty{})
	if err != nil {
		d.logger.WithError(err).Error("Disk creation failed")
		return err
	}

	d.logger.Debug("Disk created successfully")
	return nil
}

// Start initiates the driver instance and receives streaming responses. It blocks until
// receiving the initial success response, then spawns a goroutine to consume subsequent
// error messages from the stream. Any errors from the driver are sent to the channel.
func (d *DriverClient) Start(ctx context.Context) (chan error, error) {
	d.logger.Debug("Starting driver instance")

	stream, err := d.DriverSvc.Start(ctx, &emptypb.Empty{})
	if err != nil {
		d.logger.WithError(err).Error("Failed to start driver instance")
		return nil, err
	}

	// Blocking to receive an initial response to ensure Start() is initiated
	// at the server-side.
	initialResp, err := stream.Recv()
	if err != nil {
		d.logger.WithError(err).Error("Error receiving initial response from driver start")
		return nil, err
	}
	if !initialResp.Success {
		return nil, errors.New(initialResp.Error)
	}

	go func() {
		<-ctx.Done()
		if closeErr := stream.CloseSend(); closeErr != nil {
			d.logger.WithError(closeErr).Warn("Failed to close stream")
		}
	}()

	errCh := make(chan error, 1)
	go func() {
		for {
			respStream, err := stream.Recv()
			if err != nil {
				d.logger.Infof("Error receiving response from driver: %v", err)
				return
			}
			d.logger.Debugf("Received response: %v", respStream)
			if !respStream.Success {
				errCh <- errors.New(respStream.Error)
			} else {
				close(errCh)
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
		d.logger.WithError(err).Error("Failed to stop driver instance")
		return err
	}

	d.logger.Debug("Driver instance stopped successfully")
	return nil
}

func (d *DriverClient) Delete(ctx context.Context) error {
	d.logger.Debug("Deleting driver instance")

	_, err := d.DriverSvc.Delete(ctx, &emptypb.Empty{})
	if err != nil {
		d.logger.WithError(err).Error("Failed to deleted driver instance")
		return err
	}

	d.logger.Debug("Driver instance deleted successfully")
	return nil
}

func (d *DriverClient) FillConfig(_ context.Context, _ *limatype.LimaYAML, _ string) error {
	return errors.New("pre-configured driver action not implemented in client driver")
}

func (d *DriverClient) RunGUI() error {
	d.logger.Debug("Running GUI for the driver instance")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := d.DriverSvc.RunGUI(ctx, &emptypb.Empty{})
	if err != nil {
		d.logger.WithError(err).Error("Failed to run GUI")
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
		d.logger.WithError(err).Error("Failed to change display password")
		return err
	}

	d.logger.Debug("Display password changed successfully")
	return nil
}

func (d *DriverClient) DisplayConnection(ctx context.Context) (string, error) {
	d.logger.Debug("Getting display connection for the driver instance")

	resp, err := d.DriverSvc.GetDisplayConnection(ctx, &emptypb.Empty{})
	if err != nil {
		d.logger.WithError(err).Error("Failed to get display connection")
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
		d.logger.WithError(err).Error("Failed to create snapshot")
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
		d.logger.WithError(err).Error("Failed to apply snapshot")
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
		d.logger.WithError(err).Error("Failed to delete snapshot")
		return err
	}

	d.logger.Debugf("Snapshot '%s' deleted successfully", tag)
	return nil
}

func (d *DriverClient) ListSnapshots(ctx context.Context) (string, error) {
	d.logger.Debug("Listing snapshots")

	resp, err := d.DriverSvc.ListSnapshots(ctx, &emptypb.Empty{})
	if err != nil {
		d.logger.WithError(err).Error("Failed to list snapshots")
		return "", err
	}

	d.logger.Debugf("Snapshots listed successfully: %s", resp.Snapshots)
	return resp.Snapshots, nil
}

func (d *DriverClient) ForwardGuestAgent() bool {
	d.logger.Debug("Checking if guest agent needs to be forwarded")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := d.DriverSvc.ForwardGuestAgent(ctx, &emptypb.Empty{})
	if err != nil {
		d.logger.WithError(err).Error("Failed to check guest agent forwarding")
		return false
	}

	return resp.ShouldForward
}

func (d *DriverClient) GuestAgentConn(ctx context.Context) (net.Conn, string, error) {
	d.logger.Info("Getting guest agent connection")
	_, err := d.DriverSvc.GuestAgentConn(ctx, &emptypb.Empty{})
	if err != nil {
		d.logger.WithError(err).Error("Failed to get guest agent connection")
		return nil, "", err
	}

	return nil, "", nil
}

func (d *DriverClient) Info() driver.Info {
	d.logger.Debug("Getting driver info")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := d.DriverSvc.Info(ctx, &emptypb.Empty{})
	if err != nil {
		d.logger.WithError(err).Error("Failed to get driver info")
		return driver.Info{}
	}

	var info driver.Info
	if err := json.Unmarshal(resp.InfoJson, &info); err != nil {
		d.logger.WithError(err).Error("Failed to unmarshal driver info")
		return driver.Info{}
	}

	d.logger.Debugf("Driver info retrieved: %+v", info)
	return info
}

func (d *DriverClient) Configure(inst *limatype.Instance) *driver.ConfiguredDriver {
	d.logger.Debugf("Setting config for instance %s with SSH local port %d", inst.Name, inst.SSHLocalPort)

	instJSON, err := inst.MarshalJSON()
	if err != nil {
		d.logger.WithError(err).Error("Failed to marshal instance config")
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err = d.DriverSvc.Configure(ctx, &pb.SetConfigRequest{
		InstanceConfigJson: instJSON,
	})
	if err != nil {
		d.logger.WithError(err).Error("Failed to set config")
		return nil
	}

	d.logger.Debugf("Config set successfully for instance %s", inst.Name)
	return &driver.ConfiguredDriver{
		Driver: d,
	}
}

func (d *DriverClient) InspectStatus(_ context.Context, _ *limatype.Instance) string {
	return ""
}

func (d *DriverClient) SSHAddress(ctx context.Context) (string, error) {
	d.logger.Debug("Getting SSH address for the driver instance")

	resp, err := d.DriverSvc.SSHAddress(ctx, &emptypb.Empty{})
	if err != nil {
		d.logger.WithError(err).Error("Failed to get SSH address")
		return "", err
	}

	d.logger.Debugf("SSH address retrieved: %s", resp.Address)
	return resp.Address, nil
}

func (d *DriverClient) BootScripts() (map[string][]byte, error) {
	d.logger.Debug("Getting boot scripts for the driver instance")
	resp, err := d.DriverSvc.BootScripts(context.Background(), &emptypb.Empty{})
	if err != nil {
		d.logger.WithError(err).Error("Failed to get boot scripts")
		return nil, err
	}

	d.logger.Debugf("Boot scripts retrieved successfully: %d scripts", len(resp.Scripts))
	return resp.Scripts, nil
}

func (d *DriverClient) AdditionalSetupForSSH(ctx context.Context) error {
	d.logger.Debug("Performing additional setup for SSH connection")

	_, err := d.DriverSvc.AdditionalSetupForSSH(ctx, &emptypb.Empty{})
	if err != nil {
		d.logger.WithError(err).Error("Failed to perform additional setup for SSH")
		return err
	}

	d.logger.Debug("Additional setup for SSH completed successfully")
	return nil
}
