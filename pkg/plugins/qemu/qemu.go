package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/digitalocean/go-qemu/qmp"
	"github.com/digitalocean/go-qemu/qmp/raw"
	"github.com/lima-vm/lima/pkg/executil"
	"github.com/lima-vm/lima/pkg/limayaml"
	"github.com/lima-vm/lima/pkg/plugins"
	"github.com/lima-vm/lima/pkg/plugins/framework"
	"github.com/lima-vm/lima/pkg/qemu"
	"github.com/lima-vm/lima/pkg/store/filenames"
	"github.com/sirupsen/logrus"
)

// QemuPlugin is the QEMU VM driver plugin
type QemuPlugin struct {
	*framework.BasePluginServer
	instances map[string]*Instance
}

// Instance represents a running QEMU VM instance
type Instance struct {
	ID     string
	Config *framework.Config
	Status string
	qCmd   *exec.Cmd
	qWaitCh chan error
	vhostCmds []*exec.Cmd
}

// NewQemuPlugin creates a new QEMU plugin
func NewQemuPlugin() *QemuPlugin {
	return &QemuPlugin{
		BasePluginServer: framework.NewBasePluginServer(
			"qemu",
			"1.0.0",
			"QEMU VM driver plugin for Lima",
			[]string{"qemu"},
		),
		instances: make(map[string]*Instance),
	}
}

// Initialize implements the Initialize RPC
func (p *QemuPlugin) Initialize(ctx context.Context, req *plugins.InitializeRequest) (*plugins.InitializeResponse, error) {
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
func (p *QemuPlugin) CreateDisk(ctx context.Context, req *plugins.CreateDiskRequest) (*plugins.CreateDiskResponse, error) {
	config, err := framework.ParseConfig(req.Config)
	if err != nil {
		return &plugins.CreateDiskResponse{
			Success: false,
			Message: fmt.Sprintf("failed to parse config: %v", err),
		}, nil
	}

	qCfg := qemu.Config{
		Name:        config.Name,
		InstanceDir: framework.GetInstanceDir(req.InstanceId),
		LimaYAML:    &config.LimaYAML,
	}

	if err := qemu.EnsureDisk(ctx, qCfg); err != nil {
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
func (p *QemuPlugin) StartVM(ctx context.Context, req *plugins.StartVMRequest) (*plugins.StartVMResponse, error) {
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

	// Start QEMU
	qCfg := qemu.Config{
		Name:         config.Name,
		InstanceDir:  framework.GetInstanceDir(instance.ID),
		LimaYAML:     &config.LimaYAML,
		SSHLocalPort: 60022, // Default SSH port
		SSHAddress:   "127.0.0.1",
	}

	// Start virtiofsd if needed
	if *config.LimaYAML.MountType == limayaml.VIRTIOFS {
		vhostExe, err := qemu.FindVirtiofsd(qExe)
		if err != nil {
			return &plugins.StartVMResponse{
				Success: false,
				Message: fmt.Sprintf("failed to find virtiofsd: %v", err),
			}, nil
		}

		for i := range config.LimaYAML.Mounts {
			args, err := qemu.VirtiofsdCmdline(qCfg, i)
			if err != nil {
				return &plugins.StartVMResponse{
					Success: false,
					Message: fmt.Sprintf("failed to generate virtiofsd command line: %v", err),
				}, nil
			}

			vhostCmd := exec.CommandContext(ctx, vhostExe, args...)
			if err := vhostCmd.Start(); err != nil {
				return &plugins.StartVMResponse{
					Success: false,
					Message: fmt.Sprintf("failed to start virtiofsd: %v", err),
				}, nil
			}

			instance.vhostCmds = append(instance.vhostCmds, vhostCmd)
		}
	}

	qExe, qArgs, err := qemu.Cmdline(ctx, qCfg)
	if err != nil {
		return &plugins.StartVMResponse{
			Success: false,
			Message: fmt.Sprintf("failed to generate QEMU command line: %v", err),
		}, nil
	}

	// Start QEMU process
	qCmd := exec.CommandContext(ctx, qExe, qArgs...)
	qCmd.SysProcAttr = executil.BackgroundSysProcAttr

	if err := qCmd.Start(); err != nil {
		return &plugins.StartVMResponse{
			Success: false,
			Message: fmt.Sprintf("failed to start QEMU: %v", err),
		}, nil
	}

	instance.qCmd = qCmd
	instance.qWaitCh = make(chan error)
	go func() {
		instance.qWaitCh <- qCmd.Wait()
	}()

	// Wait for QEMU to start
	if err := framework.WaitForSocket(framework.GetInstanceSocketPath(instance.ID), 30*time.Second); err != nil {
		return &plugins.StartVMResponse{
			Success: false,
			Message: fmt.Sprintf("timeout waiting for QEMU to start: %v", err),
		}, nil
	}

	instance.Status = "running"

	return &plugins.StartVMResponse{
		Success: true,
		Message: "VM started successfully",
		CanRunGui: true,
	}, nil
}

// StopVM implements the StopVM RPC
func (p *QemuPlugin) StopVM(ctx context.Context, req *plugins.StopVMRequest) (*plugins.StopVMResponse, error) {
	instance, exists := p.instances[req.InstanceId]
	if !exists {
		return &plugins.StopVMResponse{
			Success: false,
			Message: "VM not found",
		}, nil
	}

	// Stop QEMU gracefully
	if err := p.shutdownQEMU(ctx, instance); err != nil {
		return &plugins.StopVMResponse{
			Success: false,
			Message: fmt.Sprintf("failed to stop QEMU: %v", err),
		}, nil
	}

	instance.Status = "stopped"
	delete(p.instances, req.InstanceId)

	return &plugins.StopVMResponse{
		Success: true,
		Message: "VM stopped successfully",
	}, nil
}

// shutdownQEMU gracefully shuts down a QEMU instance
func (p *QemuPlugin) shutdownQEMU(ctx context.Context, instance *Instance) error {
	// Connect to QMP socket
	qmpSockPath := filepath.Join(framework.GetInstanceDir(instance.ID), "qmp.sock")
	qmpClient, err := qmp.NewSocketMonitor("unix", qmpSockPath, 5*time.Second)
	if err != nil {
		return fmt.Errorf("failed to connect to QMP socket: %v", err)
	}
	defer qmpClient.Disconnect()

	if err := qmpClient.Connect(); err != nil {
		return fmt.Errorf("failed to connect to QMP: %v", err)
	}

	// Send system_powerdown command
	rawClient := raw.NewMonitor(qmpClient)
	if err := rawClient.SystemPowerdown(); err != nil {
		return fmt.Errorf("failed to send system_powerdown command: %v", err)
	}

	// Wait for QEMU to exit
	select {
	case err := <-instance.qWaitCh:
		return err
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(30 * time.Second):
		// Force kill if graceful shutdown fails
		if err := instance.qCmd.Process.Kill(); err != nil {
			return fmt.Errorf("failed to kill QEMU process: %v", err)
		}
		return <-instance.qWaitCh
	}
}

// GetGuestAgentConnection implements the GetGuestAgentConnection RPC
func (p *QemuPlugin) GetGuestAgentConnection(ctx context.Context, req *plugins.GetGuestAgentConnectionRequest) (*plugins.GetGuestAgentConnectionResponse, error) {
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

// CreateSnapshot implements the CreateSnapshot RPC
func (p *QemuPlugin) CreateSnapshot(ctx context.Context, req *plugins.CreateSnapshotRequest) (*plugins.CreateSnapshotResponse, error) {
	instance, exists := p.instances[req.InstanceId]
	if !exists {
		return &plugins.CreateSnapshotResponse{
			Success: false,
			Message: "VM not found",
		}, nil
	}

	qCfg := qemu.Config{
		Name:        instance.ID,
		InstanceDir: framework.GetInstanceDir(req.InstanceId),
		LimaYAML:    &instance.Config.LimaYAML,
	}

	if err := qemu.Save(qCfg, instance.Status == "running", req.Tag); err != nil {
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
func (p *QemuPlugin) DeleteSnapshot(ctx context.Context, req *plugins.DeleteSnapshotRequest) (*plugins.DeleteSnapshotResponse, error) {
	instance, exists := p.instances[req.InstanceId]
	if !exists {
		return &plugins.DeleteSnapshotResponse{
			Success: false,
			Message: "VM not found",
		}, nil
	}

	qCfg := qemu.Config{
		Name:        instance.ID,
		InstanceDir: framework.GetInstanceDir(req.InstanceId),
		LimaYAML:    &instance.Config.LimaYAML,
	}

	if err := qemu.Del(qCfg, instance.Status == "running", req.Tag); err != nil {
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
func (p *QemuPlugin) ApplySnapshot(ctx context.Context, req *plugins.ApplySnapshotRequest) (*plugins.ApplySnapshotResponse, error) {
	instance, exists := p.instances[req.InstanceId]
	if !exists {
		return &plugins.ApplySnapshotResponse{
			Success: false,
			Message: "VM not found",
		}, nil
	}

	qCfg := qemu.Config{
		Name:        instance.ID,
		InstanceDir: framework.GetInstanceDir(req.InstanceId),
		LimaYAML:    &instance.Config.LimaYAML,
	}

	if err := qemu.Load(qCfg, instance.Status == "running", req.Tag); err != nil {
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
func (p *QemuPlugin) ListSnapshots(ctx context.Context, req *plugins.ListSnapshotsRequest) (*plugins.ListSnapshotsResponse, error) {
	instance, exists := p.instances[req.InstanceId]
	if !exists {
		return &plugins.ListSnapshotsResponse{
			Success: false,
			Message: "VM not found",
		}, nil
	}

	qCfg := qemu.Config{
		Name:        instance.ID,
		InstanceDir: framework.GetInstanceDir(req.InstanceId),
		LimaYAML:    &instance.Config.LimaYAML,
	}

	snapshots, err := qemu.List(qCfg, instance.Status == "running")
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
func (p *QemuPlugin) ChangeDisplayPassword(ctx context.Context, req *plugins.ChangeDisplayPasswordRequest) (*plugins.ChangeDisplayPasswordResponse, error) {
	instance, exists := p.instances[req.InstanceId]
	if !exists {
		return &plugins.ChangeDisplayPasswordResponse{
			Success: false,
			Message: "VM not found",
		}, nil
	}

	qmpSockPath := filepath.Join(framework.GetInstanceDir(req.InstanceId), filenames.QMPSock)
	qmpClient, err := qmp.NewSocketMonitor("unix", qmpSockPath, 5*time.Second)
	if err != nil {
		return &plugins.ChangeDisplayPasswordResponse{
			Success: false,
			Message: fmt.Sprintf("failed to connect to QMP socket: %v", err),
		}, nil
	}
	defer qmpClient.Disconnect()

	if err := qmpClient.Connect(); err != nil {
		return &plugins.ChangeDisplayPasswordResponse{
			Success: false,
			Message: fmt.Sprintf("failed to connect to QMP: %v", err),
		}, nil
	}

	rawClient := raw.NewMonitor(qmpClient)
	if err := rawClient.ChangeVNCPassword(req.Password); err != nil {
		return &plugins.ChangeDisplayPasswordResponse{
			Success: false,
			Message: fmt.Sprintf("failed to change VNC password: %v", err),
		}, nil
	}

	return &plugins.ChangeDisplayPasswordResponse{
		Success: true,
		Message: "Display password changed successfully",
	}, nil
}

// GetDisplayConnection implements the GetDisplayConnection RPC
func (p *QemuPlugin) GetDisplayConnection(ctx context.Context, req *plugins.GetDisplayConnectionRequest) (*plugins.GetDisplayConnectionResponse, error) {
	instance, exists := p.instances[req.InstanceId]
	if !exists {
		return &plugins.GetDisplayConnectionResponse{
			Success: false,
			Message: "VM not found",
		}, nil
	}

	qmpSockPath := filepath.Join(framework.GetInstanceDir(req.InstanceId), filenames.QMPSock)
	qmpClient, err := qmp.NewSocketMonitor("unix", qmpSockPath, 5*time.Second)
	if err != nil {
		return &plugins.GetDisplayConnectionResponse{
			Success: false,
			Message: fmt.Sprintf("failed to connect to QMP socket: %v", err),
		}, nil
	}
	defer qmpClient.Disconnect()

	if err := qmpClient.Connect(); err != nil {
		return &plugins.GetDisplayConnectionResponse{
			Success: false,
			Message: fmt.Sprintf("failed to connect to QMP: %v", err),
		}, nil
	}

	rawClient := raw.NewMonitor(qmpClient)
	info, err := rawClient.QueryVNC()
	if err != nil {
		return &plugins.GetDisplayConnectionResponse{
			Success: false,
			Message: fmt.Sprintf("failed to query VNC info: %v", err),
		}, nil
	}

	return &plugins.GetDisplayConnectionResponse{
		Success: true,
		Message: "Display connection info retrieved successfully",
		Connection: *info.Service,
	}, nil
}

func main() {
	plugin := NewQemuPlugin()
	socketPath := framework.GetPluginSocketPath("qemu")
	if err := plugin.Start(socketPath); err != nil {
		log.Fatalf("Failed to start plugin: %v", err)
	}
} 