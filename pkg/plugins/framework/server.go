package framework

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/lima-vm/lima/pkg/plugins"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

// BasePluginServer provides a base implementation for VM driver plugins
type BasePluginServer struct {
	plugins.UnimplementedVMDriverServer
	metadata *plugins.GetMetadataResponse
	server   *grpc.Server
}

// NewBasePluginServer creates a new base plugin server
func NewBasePluginServer(name, version, description string, supportedVMTypes []string) *BasePluginServer {
	return &BasePluginServer{
		metadata: &plugins.GetMetadataResponse{
			Name:             name,
			Version:          version,
			Description:      description,
			SupportedVMTypes: supportedVMTypes,
		},
	}
}

// Start starts the plugin server on the given Unix socket path
func (s *BasePluginServer) Start(socketPath string) error {
	// Remove existing socket file if it exists
	if err := os.RemoveAll(socketPath); err != nil {
		return fmt.Errorf("failed to remove existing socket: %w", err)
	}

	// Create Unix domain socket listener
	lis, err := net.Listen("unix", socketPath)
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}

	// Create gRPC server
	s.server = grpc.NewServer()
	plugins.RegisterVMDriverServer(s.server, s)
	reflection.Register(s.server)

	// Handle graceful shutdown
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		s.server.GracefulStop()
		os.Exit(0)
	}()

	// Start serving
	if err := s.server.Serve(lis); err != nil {
		return fmt.Errorf("failed to serve: %w", err)
	}

	return nil
}

// Stop gracefully stops the plugin server
func (s *BasePluginServer) Stop() {
	if s.server != nil {
		s.server.GracefulStop()
	}
}

// GetMetadata implements the GetMetadata RPC
func (s *BasePluginServer) GetMetadata(ctx context.Context, req *plugins.GetMetadataRequest) (*plugins.GetMetadataResponse, error) {
	return s.metadata, nil
}

// StartVM implements the StartVM RPC
func (s *BasePluginServer) StartVM(ctx context.Context, req *plugins.StartVMRequest) (*plugins.StartVMResponse, error) {
	return nil, fmt.Errorf("not implemented")
}

// StopVM implements the StopVM RPC
func (s *BasePluginServer) StopVM(ctx context.Context, req *plugins.StopVMRequest) (*plugins.StopVMResponse, error) {
	return nil, fmt.Errorf("not implemented")
}

// Initialize implements the Initialize RPC
func (s *BasePluginServer) Initialize(ctx context.Context, req *plugins.InitializeRequest) (*plugins.InitializeResponse, error) {
	return nil, fmt.Errorf("not implemented")
}

// CreateDisk implements the CreateDisk RPC
func (s *BasePluginServer) CreateDisk(ctx context.Context, req *plugins.CreateDiskRequest) (*plugins.CreateDiskResponse, error) {
	return nil, fmt.Errorf("not implemented")
}

// Validate implements the Validate RPC
func (s *BasePluginServer) Validate(ctx context.Context, req *plugins.ValidateRequest) (*plugins.ValidateResponse, error) {
	return nil, fmt.Errorf("not implemented")
}

// Register implements the Register RPC
func (s *BasePluginServer) Register(ctx context.Context, req *plugins.RegisterRequest) (*plugins.RegisterResponse, error) {
	return nil, fmt.Errorf("not implemented")
}

// Unregister implements the Unregister RPC
func (s *BasePluginServer) Unregister(ctx context.Context, req *plugins.UnregisterRequest) (*plugins.UnregisterResponse, error) {
	return nil, fmt.Errorf("not implemented")
}

// ChangeDisplayPassword implements the ChangeDisplayPassword RPC
func (s *BasePluginServer) ChangeDisplayPassword(ctx context.Context, req *plugins.ChangeDisplayPasswordRequest) (*plugins.ChangeDisplayPasswordResponse, error) {
	return nil, fmt.Errorf("not implemented")
}

// GetDisplayConnection implements the GetDisplayConnection RPC
func (s *BasePluginServer) GetDisplayConnection(ctx context.Context, req *plugins.GetDisplayConnectionRequest) (*plugins.GetDisplayConnectionResponse, error) {
	return nil, fmt.Errorf("not implemented")
}

// CreateSnapshot implements the CreateSnapshot RPC
func (s *BasePluginServer) CreateSnapshot(ctx context.Context, req *plugins.CreateSnapshotRequest) (*plugins.CreateSnapshotResponse, error) {
	return nil, fmt.Errorf("not implemented")
}

// ApplySnapshot implements the ApplySnapshot RPC
func (s *BasePluginServer) ApplySnapshot(ctx context.Context, req *plugins.ApplySnapshotRequest) (*plugins.ApplySnapshotResponse, error) {
	return nil, fmt.Errorf("not implemented")
}

// DeleteSnapshot implements the DeleteSnapshot RPC
func (s *BasePluginServer) DeleteSnapshot(ctx context.Context, req *plugins.DeleteSnapshotRequest) (*plugins.DeleteSnapshotResponse, error) {
	return nil, fmt.Errorf("not implemented")
}

// ListSnapshots implements the ListSnapshots RPC
func (s *BasePluginServer) ListSnapshots(ctx context.Context, req *plugins.ListSnapshotsRequest) (*plugins.ListSnapshotsResponse, error) {
	return nil, fmt.Errorf("not implemented")
}

// GetGuestAgentConnection implements the GetGuestAgentConnection RPC
func (s *BasePluginServer) GetGuestAgentConnection(ctx context.Context, req *plugins.GetGuestAgentConnectionRequest) (*plugins.GetGuestAgentConnectionResponse, error) {
	return nil, fmt.Errorf("not implemented")
} 