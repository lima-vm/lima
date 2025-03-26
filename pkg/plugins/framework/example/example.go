package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/lima-vm/lima/pkg/plugins"
	"github.com/lima-vm/lima/pkg/plugins/framework"
)

// ExamplePlugin is an example VM driver plugin
type ExamplePlugin struct {
	*framework.BasePluginServer
	instances map[string]*Instance
}

// Instance represents a running VM instance
type Instance struct {
	ID     string
	Config *framework.Config
	Status string
}

// NewExamplePlugin creates a new example plugin
func NewExamplePlugin() *ExamplePlugin {
	return &ExamplePlugin{
		BasePluginServer: framework.NewBasePluginServer(
			"example",
			"1.0.0",
			"An example VM driver plugin",
			[]string{"example-vm"},
		),
		instances: make(map[string]*Instance),
	}
}

// Initialize implements the Initialize RPC
func (p *ExamplePlugin) Initialize(ctx context.Context, req *plugins.InitializeRequest) (*plugins.InitializeResponse, error) {
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
func (p *ExamplePlugin) CreateDisk(ctx context.Context, req *plugins.CreateDiskRequest) (*plugins.CreateDiskResponse, error) {
	config, err := framework.ParseConfig(req.Config)
	if err != nil {
		return &plugins.CreateDiskResponse{
			Success: false,
			Message: fmt.Sprintf("failed to parse config: %v", err),
		}, nil
	}

	diskPath := framework.GetInstanceDiskPath(req.InstanceId)
	if err := framework.CreateDiskImage(diskPath, *config.Disk); err != nil {
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
func (p *ExamplePlugin) StartVM(ctx context.Context, req *plugins.StartVMRequest) (*plugins.StartVMResponse, error) {
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

	// Example: Start a background process to simulate VM startup
	go func() {
		time.Sleep(2 * time.Second) // Simulate startup time
		instance.Status = "running"
	}()

	return &plugins.StartVMResponse{
		Success: true,
		Message: "VM started successfully",
		CanRunGui: false,
	}, nil
}

// StopVM implements the StopVM RPC
func (p *ExamplePlugin) StopVM(ctx context.Context, req *plugins.StopVMRequest) (*plugins.StopVMResponse, error) {
	instance, exists := p.instances[req.InstanceId]
	if !exists {
		return &plugins.StopVMResponse{
			Success: false,
			Message: "VM not found",
		}, nil
	}

	// Example: Stop the VM
	instance.Status = "stopped"
	delete(p.instances, req.InstanceId)

	return &plugins.StopVMResponse{
		Success: true,
		Message: "VM stopped successfully",
	}, nil
}

// CreateSnapshot implements the CreateSnapshot RPC
func (p *ExamplePlugin) CreateSnapshot(ctx context.Context, req *plugins.CreateSnapshotRequest) (*plugins.CreateSnapshotResponse, error) {
	instance, exists := p.instances[req.InstanceId]
	if !exists {
		return &plugins.CreateSnapshotResponse{
			Success: false,
			Message: "VM not found",
		}, nil
	}

	diskPath := framework.GetInstanceDiskPath(req.InstanceId)
	snapshotPath := filepath.Join(framework.GetInstanceDir(req.InstanceId), fmt.Sprintf("snapshot-%s.qcow2", req.Tag))

	if err := framework.CreateSnapshot(diskPath, snapshotPath); err != nil {
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
func (p *ExamplePlugin) DeleteSnapshot(ctx context.Context, req *plugins.DeleteSnapshotRequest) (*plugins.DeleteSnapshotResponse, error) {
	snapshotPath := filepath.Join(framework.GetInstanceDir(req.InstanceId), fmt.Sprintf("snapshot-%s.qcow2", req.Tag))
	if err := framework.DeleteSnapshot(snapshotPath); err != nil {
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

// GetGuestAgentConnection implements the GetGuestAgentConnection RPC
func (p *ExamplePlugin) GetGuestAgentConnection(ctx context.Context, req *plugins.GetGuestAgentConnectionRequest) (*plugins.GetGuestAgentConnectionResponse, error) {
	instance, exists := p.instances[req.InstanceId]
	if !exists {
		return &plugins.GetGuestAgentConnectionResponse{
			Success: false,
			Message: "VM not found",
		}, nil
	}

	return &plugins.GetGuestAgentConnectionResponse{
		Success: true,
		Message: "Guest agent connection info retrieved successfully",
		ForwardGuestAgent: true,
		ConnectionAddress: fmt.Sprintf("unix://%s", framework.GetInstanceSocketPath(req.InstanceId)),
	}, nil
}

func main() {
	plugin := NewExamplePlugin()
	socketPath := framework.GetPluginSocketPath("example")
	if err := plugin.Start(socketPath); err != nil {
		log.Fatalf("Failed to start plugin: %v", err)
	}
} 