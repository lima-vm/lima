package plugins

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Manager handles plugin discovery and lifecycle management
type Manager struct {
	mu      sync.RWMutex
	plugins map[string]*Plugin
}

// Plugin represents a loaded VM driver plugin
type Plugin struct {
	name     string
	path     string
	conn     *grpc.ClientConn
	client   VMDriverClient
	metadata *PluginMetadata
}

// PluginMetadata contains information about the plugin
type PluginMetadata struct {
	Name        string
	Version     string
	Description string
	SupportedVMTypes []string
}

// NewManager creates a new plugin manager
func NewManager() *Manager {
	return &Manager{
		plugins: make(map[string]*Plugin),
	}
}

// LoadPlugin loads a plugin from the given path
func (m *Manager) LoadPlugin(ctx context.Context, path string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if plugin is already loaded
	if _, exists := m.plugins[path]; exists {
		return fmt.Errorf("plugin already loaded: %s", path)
	}

	// Create gRPC connection to the plugin
	conn, err := grpc.Dial("unix://"+path,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
		grpc.WithTimeout(defaultTimeout),
	)
	if err != nil {
		return fmt.Errorf("failed to connect to plugin: %w", err)
	}

	// Create plugin client
	client := NewVMDriverClient(conn)

	// Get plugin metadata
	metadata, err := client.GetMetadata(ctx, &GetMetadataRequest{})
	if err != nil {
		conn.Close()
		return fmt.Errorf("failed to get plugin metadata: %w", err)
	}

	plugin := &Plugin{
		name:     metadata.Name,
		path:     path,
		conn:     conn,
		client:   client,
		metadata: metadata,
	}

	m.plugins[path] = plugin
	return nil
}

// UnloadPlugin unloads a plugin
func (m *Manager) UnloadPlugin(path string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	plugin, exists := m.plugins[path]
	if !exists {
		return fmt.Errorf("plugin not found: %s", path)
	}

	if err := plugin.conn.Close(); err != nil {
		return fmt.Errorf("failed to close plugin connection: %w", err)
	}

	delete(m.plugins, path)
	return nil
}

// GetPlugin returns a plugin by name
func (m *Manager) GetPlugin(name string) (*Plugin, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, plugin := range m.plugins {
		if plugin.name == name {
			return plugin, nil
		}
	}
	return nil, fmt.Errorf("plugin not found: %s", name)
}

// DiscoverPlugins discovers plugins in the plugin directory
func (m *Manager) DiscoverPlugins(ctx context.Context, pluginDir string) error {
	entries, err := os.ReadDir(pluginDir)
	if err != nil {
		return fmt.Errorf("failed to read plugin directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		// Check if file is executable
		path := filepath.Join(pluginDir, entry.Name())
		if _, err := os.Stat(path); err != nil {
			continue
		}

		// Try to load the plugin
		if err := m.LoadPlugin(ctx, path); err != nil {
			// Log error but continue with other plugins
			fmt.Printf("Failed to load plugin %s: %v\n", path, err)
			continue
		}
	}

	return nil
}

// Close closes all plugin connections
func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, plugin := range m.plugins {
		if err := plugin.conn.Close(); err != nil {
			return fmt.Errorf("failed to close plugin connection: %w", err)
		}
	}

	m.plugins = make(map[string]*Plugin)
	return nil
} 