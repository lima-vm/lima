package framework

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/lima-vm/lima/pkg/limayaml"
)

// Config represents the VM configuration
type Config struct {
	limayaml.LimaYAML
}

// ParseConfig parses the serialized config into a Config struct
func ParseConfig(serializedConfig string) (*Config, error) {
	var config Config
	if err := json.Unmarshal([]byte(serializedConfig), &config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}
	return &config, nil
}

// CreateResponse creates a success response
func CreateResponse(success bool, message string) *plugins.Response {
	return &plugins.Response{
		Success: success,
		Message: message,
	}
}

// CreateErrorResponse creates an error response
func CreateErrorResponse(err error) *plugins.Response {
	return &plugins.Response{
		Success: false,
		Message: err.Error(),
	}
}

// EnsureDir ensures a directory exists, creating it if necessary
func EnsureDir(path string) error {
	if err := os.MkdirAll(path, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", path, err)
	}
	return nil
}

// GetInstanceDir returns the instance directory path
func GetInstanceDir(instanceID string) string {
	return filepath.Join(os.Getenv("HOME"), ".lima", instanceID)
}

// GetInstanceConfigPath returns the instance config file path
func GetInstanceConfigPath(instanceID string) string {
	return filepath.Join(GetInstanceDir(instanceID), "lima.yaml")
}

// GetInstanceDiskPath returns the instance disk file path
func GetInstanceDiskPath(instanceID string) string {
	return filepath.Join(GetInstanceDir(instanceID), "disk.img")
}

// GetInstanceSocketPath returns the instance socket file path
func GetInstanceSocketPath(instanceID string) string {
	return filepath.Join(GetInstanceDir(instanceID), "socket")
}

// GetPluginSocketPath returns the plugin socket file path
func GetPluginSocketPath(pluginName string) string {
	return filepath.Join("/tmp", fmt.Sprintf("lima-plugin-%s.sock", pluginName))
}

// CreateDiskImage creates a disk image with the given size
func CreateDiskImage(path string, sizeGB int) error {
	cmd := exec.Command("qemu-img", "create", "-f", "qcow2", path, fmt.Sprintf("%dG", sizeGB))
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to create disk image: %s: %w", string(output), err)
	}
	return nil
}

// ResizeDiskImage resizes a disk image to the given size
func ResizeDiskImage(path string, sizeGB int) error {
	cmd := exec.Command("qemu-img", "resize", path, fmt.Sprintf("%dG", sizeGB))
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to resize disk image: %s: %w", string(output), err)
	}
	return nil
}

// CreateSnapshot creates a snapshot of a disk image
func CreateSnapshot(path, snapshotPath string) error {
	cmd := exec.Command("qemu-img", "create", "-f", "qcow2", "-b", path, snapshotPath)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to create snapshot: %s: %w", string(output), err)
	}
	return nil
}

// DeleteSnapshot deletes a snapshot
func DeleteSnapshot(snapshotPath string) error {
	if err := os.Remove(snapshotPath); err != nil {
		return fmt.Errorf("failed to delete snapshot: %w", err)
	}
	return nil
}

// WaitForSocket waits for a Unix socket to become available
func WaitForSocket(path string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			return nil
		}
		<-ticker.C
	}
	return fmt.Errorf("timeout waiting for socket %s", path)
}

// WriteConfig writes the config to a file
func WriteConfig(config *Config, path string) error {
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}
	return nil
}

// ReadConfig reads the config from a file
func ReadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}
	return ParseConfig(string(data))
}

// ValidateConfig validates the VM configuration
func ValidateConfig(config *Config) error {
	if config.VMType == nil {
		return fmt.Errorf("vmType is required")
	}
	if config.CPUs == nil {
		return fmt.Errorf("cpus is required")
	}
	if config.Memory == nil {
		return fmt.Errorf("memory is required")
	}
	if config.Disk == nil {
		return fmt.Errorf("disk is required")
	}
	return nil
}

// GetSSHConfig returns the SSH configuration for the VM
func GetSSHConfig(instanceID string) string {
	return fmt.Sprintf(`Host lima-%s
  HostName 127.0.0.1
  Port 60022
  User lima
  IdentityFile ~/.lima/%s/id_ed25519
  StrictHostKeyChecking no
  UserKnownHostsFile /dev/null`, instanceID, instanceID)
} 