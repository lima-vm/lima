package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/Code-Hex/vz/v3"
	"github.com/lima-vm/lima/pkg/limayaml"
	"github.com/lima-vm/lima/pkg/plugins"
	"github.com/lima-vm/lima/pkg/plugins/framework"
	"github.com/lima-vm/lima/pkg/vz"
	"github.com/sirupsen/logrus"
)

// VzPlugin is the VZ VM driver plugin
type VzPlugin struct {
	*framework.BasePluginServer
	instances map[string]*Instance
}

// Instance represents a running VZ VM instance
type Instance struct {
	ID     string
	Config *framework.Config
	Status string
	vm     *vz.VirtualMachine
	mu     sync.Mutex
}

// NewVzPlugin creates a new VZ plugin
func NewVzPlugin() *VzPlugin {
	return &VzPlugin{
		BasePluginServer: framework.NewBasePluginServer(
			"vz",
			"1.0.0",
			"VZ VM driver plugin for Lima",
			[]string{"vz"},
		),
		instances: make(map[string]*Instance),
	}
}

// Initialize implements the Initialize RPC
func (p *VzPlugin) Initialize(ctx context.Context, req *plugins.InitializeRequest) (*plugins.InitializeResponse, error) {
	config, err := framework.ParseConfig(req.Config)
	if err != nil {
		return &plugins.InitializeResponse{
			Success: false,
			Message: fmt.Sprintf("failed to parse config: %v", err),
		}, nil
	}

	// Validate config
	if err := framework.ValidateConfig(config); err != nil {
		return &plugins.InitializeResponse{
			Success: false,
			Message: fmt.Sprintf("invalid config: %v", err),
		}, nil
	}

	// Create instance directory
	instanceDir := framework.GetInstanceDir(req.InstanceId)
	if err := framework.EnsureDir(instanceDir); err != nil {
		return &plugins.InitializeResponse{
			Success: false,
			Message: fmt.Sprintf("failed to create instance directory: %v", err),
		}, nil
	}

	// Write config file
	configPath := framework.GetInstanceConfigPath(req.InstanceId)
	if err := framework.WriteConfig(config, configPath); err != nil {
		return &plugins.InitializeResponse{
			Success: false,
			Message: fmt.Sprintf("failed to write config: %v", err),
		}, nil
	}

	return &plugins.InitializeResponse{
		Success: true,
		Message: "Instance initialized successfully",
	}, nil
}

// CreateDisk implements the CreateDisk RPC
func (p *VzPlugin) CreateDisk(ctx context.Context, req *plugins.CreateDiskRequest) (*plugins.CreateDiskResponse, error) {
	config, err := framework.ParseConfig(req.Config)
	if err != nil {
		return &plugins.CreateDiskResponse{
			Success: false,
			Message: fmt.Sprintf("failed to parse config: %v", err),
		}, nil
	}

	// Create base driver for disk creation
	baseDriver := &driver.BaseDriver{
		Instance: &store.Instance{
			Dir:    framework.GetInstanceDir(req.InstanceId),
			Config: &config.LimaYAML,
		},
	}

	if err := vz.EnsureDisk(ctx, baseDriver); err != nil {
		return &plugins.CreateDiskResponse{
			Success: false,
			Message: fmt.Sprintf("failed to create disk: %v", err),
		}, nil
	}

	return &plugins.CreateDiskResponse{
		Success: true,
		Message: "Disk created successfully",
	}, nil
}

// StartVM implements the StartVM RPC
func (p *VzPlugin) StartVM(ctx context.Context, req *plugins.StartVMRequest) (*plugins.StartVMResponse, error) {
	config, err := framework.ParseConfig(req.Config)
	if err != nil {
		return &plugins.StartVMResponse{
			Success: false,
			Message: fmt.Sprintf("failed to parse config: %v", err),
		}, nil
	}

	// Create instance
	instance := &Instance{
		ID:     config.Name,
		Config: config,
		Status: "starting",
	}

	// Store instance
	p.instances[instance.ID] = instance

	// Create base driver for VM creation
	baseDriver := &driver.BaseDriver{
		Instance: &store.Instance{
			Dir:    framework.GetInstanceDir(instance.ID),
			Config: &config.LimaYAML,
		},
	}

	// Start VM
	vm, errCh, err := vz.StartVM(ctx, baseDriver)
	if err != nil {
		return &plugins.StartVMResponse{
			Success: false,
			Message: fmt.Sprintf("failed to start VM: %v", err),
		}, nil
	}

	instance.vm = vm

	// Wait for VM to be ready
	go func() {
		select {
		case err := <-errCh:
			if err != nil {
				logrus.Errorf("VM error: %v", err)
			}
		case <-ctx.Done():
		}
	}()

	// Wait for VM to be running
	timeout := time.After(30 * time.Second)
	ticker := time.NewTicker(500 * time.Millisecond)
	for {
		select {
		case <-timeout:
			return &plugins.StartVMResponse{
				Success: false,
				Message: "timeout waiting for VM to start",
			}, nil
		case <-ticker.C:
			if instance.vm.State() == vz.VirtualMachineStateRunning {
				instance.Status = "running"
				return &plugins.StartVMResponse{
					Success: true,
					Message: "VM started successfully",
					CanRunGui: true,
				}, nil
			}
		}
	}
}

// StopVM implements the StopVM RPC
func (p *VzPlugin) StopVM(ctx context.Context, req *plugins.StopVMRequest) (*plugins.StopVMResponse, error) {
	instance, exists := p.instances[req.InstanceId]
	if !exists {
		return &plugins.StopVMResponse{
			Success: false,
			Message: "VM not found",
		}, nil
	}

	// Stop VM gracefully
	if err := p.shutdownVM(ctx, instance); err != nil {
		return &plugins.StopVMResponse{
			Success: false,
			Message: fmt.Sprintf("failed to stop VM: %v", err),
		}, nil
	}

	instance.Status = "stopped"
	delete(p.instances, req.InstanceId)

	return &plugins.StopVMResponse{
		Success: true,
		Message: "VM stopped successfully",
	}, nil
}

// shutdownVM gracefully shuts down a VZ VM instance
func (p *VzPlugin) shutdownVM(ctx context.Context, instance *Instance) error {
	canStop := instance.vm.CanRequestStop()
	if !canStop {
		return errors.New("VM does not support graceful shutdown")
	}

	_, err := instance.vm.RequestStop()
	if err != nil {
		return fmt.Errorf("failed to request VM stop: %v", err)
	}

	timeout := time.After(5 * time.Second)
	ticker := time.NewTicker(500 * time.Millisecond)
	for {
		select {
		case <-timeout:
			return errors.New("timeout waiting for VM to stop")
		case <-ticker.C:
			if instance.vm.State() == vz.VirtualMachineStateStopped {
				return nil
			}
		}
	}
}

// GetGuestAgentConnection implements the GetGuestAgentConnection RPC
func (p *VzPlugin) GetGuestAgentConnection(ctx context.Context, req *plugins.GetGuestAgentConnectionRequest) (*plugins.GetGuestAgentConnectionResponse, error) {
	instance, exists := p.instances[req.InstanceId]
	if !exists {
		return &plugins.GetGuestAgentConnectionResponse{
			Success: false,
			Message: "VM not found",
		}, nil
	}

	// Get guest agent connection
	conn, err := instance.vm.GuestAgentConn(ctx)
	if err != nil {
		return &plugins.GetGuestAgentConnectionResponse{
			Success: false,
			Message: fmt.Sprintf("failed to get guest agent connection: %v", err),
		}, nil
	}

	return &plugins.GetGuestAgentConnectionResponse{
		Success: true,
		Message: "Guest agent connection info retrieved successfully",
		ForwardGuestAgent: true,
		ConnectionAddress: fmt.Sprintf("unix://%s", framework.GetInstanceSocketPath(req.InstanceId)),
	}, nil
}

// CreateSnapshot implements the CreateSnapshot RPC
func (p *VzPlugin) CreateSnapshot(ctx context.Context, req *plugins.CreateSnapshotRequest) (*plugins.CreateSnapshotResponse, error) {
	instance, exists := p.instances[req.InstanceId]
	if !exists {
		return &plugins.CreateSnapshotResponse{
			Success: false,
			Message: "VM not found",
		}, nil
	}

	// Create base driver for snapshot creation
	baseDriver := &driver.BaseDriver{
		Instance: &store.Instance{
			Dir:    framework.GetInstanceDir(req.InstanceId),
			Config: &instance.Config.LimaYAML,
		},
	}

	if err := vz.CreateSnapshot(baseDriver, req.Tag); err != nil {
		return &plugins.CreateSnapshotResponse{
			Success: false,
			Message: fmt.Sprintf("failed to create snapshot: %v", err),
		}, nil
	}

	return &plugins.CreateSnapshotResponse{
		Success: true,
		Message: "Snapshot created successfully",
	}, nil
}

// DeleteSnapshot implements the DeleteSnapshot RPC
func (p *VzPlugin) DeleteSnapshot(ctx context.Context, req *plugins.DeleteSnapshotRequest) (*plugins.DeleteSnapshotResponse, error) {
	instance, exists := p.instances[req.InstanceId]
	if !exists {
		return &plugins.DeleteSnapshotResponse{
			Success: false,
			Message: "VM not found",
		}, nil
	}

	// Create base driver for snapshot deletion
	baseDriver := &driver.BaseDriver{
		Instance: &store.Instance{
			Dir:    framework.GetInstanceDir(req.InstanceId),
			Config: &instance.Config.LimaYAML,
		},
	}

	if err := vz.DeleteSnapshot(baseDriver, req.Tag); err != nil {
		return &plugins.DeleteSnapshotResponse{
			Success: false,
			Message: fmt.Sprintf("failed to delete snapshot: %v", err),
		}, nil
	}

	return &plugins.DeleteSnapshotResponse{
		Success: true,
		Message: "Snapshot deleted successfully",
	}, nil
}

// ApplySnapshot implements the ApplySnapshot RPC
func (p *VzPlugin) ApplySnapshot(ctx context.Context, req *plugins.ApplySnapshotRequest) (*plugins.ApplySnapshotResponse, error) {
	instance, exists := p.instances[req.InstanceId]
	if !exists {
		return &plugins.ApplySnapshotResponse{
			Success: false,
			Message: "VM not found",
		}, nil
	}

	// Create base driver for snapshot application
	baseDriver := &driver.BaseDriver{
		Instance: &store.Instance{
			Dir:    framework.GetInstanceDir(req.InstanceId),
			Config: &instance.Config.LimaYAML,
		},
	}

	if err := vz.ApplySnapshot(baseDriver, req.Tag); err != nil {
		return &plugins.ApplySnapshotResponse{
			Success: false,
			Message: fmt.Sprintf("failed to apply snapshot: %v", err),
		}, nil
	}

	return &plugins.ApplySnapshotResponse{
		Success: true,
		Message: "Snapshot applied successfully",
	}, nil
}

// ListSnapshots implements the ListSnapshots RPC
func (p *VzPlugin) ListSnapshots(ctx context.Context, req *plugins.ListSnapshotsRequest) (*plugins.ListSnapshotsResponse, error) {
	instance, exists := p.instances[req.InstanceId]
	if !exists {
		return &plugins.ListSnapshotsResponse{
			Success: false,
			Message: "VM not found",
		}, nil
	}

	// Create base driver for snapshot listing
	baseDriver := &driver.BaseDriver{
		Instance: &store.Instance{
			Dir:    framework.GetInstanceDir(req.InstanceId),
			Config: &instance.Config.LimaYAML,
		},
	}

	snapshots, err := vz.ListSnapshots(baseDriver)
	if err != nil {
		return &plugins.ListSnapshotsResponse{
			Success: false,
			Message: fmt.Sprintf("failed to list snapshots: %v", err),
		}, nil
	}

	return &plugins.ListSnapshotsResponse{
		Success: true,
		Message: "Snapshots listed successfully",
		Snapshots: snapshots,
	}, nil
}

// ChangeDisplayPassword implements the ChangeDisplayPassword RPC
func (p *VzPlugin) ChangeDisplayPassword(ctx context.Context, req *plugins.ChangeDisplayPasswordRequest) (*plugins.ChangeDisplayPasswordResponse, error) {
	instance, exists := p.instances[req.InstanceId]
	if !exists {
		return &plugins.ChangeDisplayPasswordResponse{
			Success: false,
			Message: "VM not found",
		}, nil
	}

	// VZ driver does not support display password changes
	return &plugins.ChangeDisplayPasswordResponse{
		Success: false,
		Message: "Display password changes are not supported by the VZ driver",
	}, nil
}

// GetDisplayConnection implements the GetDisplayConnection RPC
func (p *VzPlugin) GetDisplayConnection(ctx context.Context, req *plugins.GetDisplayConnectionRequest) (*plugins.GetDisplayConnectionResponse, error) {
	instance, exists := p.instances[req.InstanceId]
	if !exists {
		return &plugins.GetDisplayConnectionResponse{
			Success: false,
			Message: "VM not found",
		}, nil
	}

	// VZ driver does not support display connections
	return &plugins.GetDisplayConnectionResponse{
		Success: false,
		Message: "Display connections are not supported by the VZ driver",
	}, nil
}

func main() {
	plugin := NewVzPlugin()
	socketPath := framework.GetPluginSocketPath("vz")
	if err := plugin.Start(socketPath); err != nil {
		log.Fatalf("Failed to start plugin: %v", err)
	}
} 