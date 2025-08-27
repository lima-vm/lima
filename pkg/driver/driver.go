// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package driver

import (
	"context"
	"net"

	"github.com/lima-vm/lima/v2/pkg/limatype"
)

// Lifecycle defines basic lifecycle operations.
type Lifecycle interface {
	// Validate returns error if the current driver isn't support for given config
	Validate(_ context.Context) error

	// Create is called on creating the instance for the first time.
	// (e.g., creating "vz-identifier" file)
	//
	// Create MUST return nil when it is called against an existing instance.
	//
	// Create does not create the disks.
	Create(_ context.Context) error

	// CreateDisk returns error if the current driver fails in creating disk
	CreateDisk(_ context.Context) error

	// Start is used for booting the vm using driver instance
	// It returns a chan error on successful boot
	// The second argument may contain error occurred while starting driver
	Start(_ context.Context) (chan error, error)

	// Stop will terminate the running vm instance.
	// It returns error if there are any errors during Stop
	Stop(_ context.Context) error

	Delete(_ context.Context) error

	InspectStatus(_ context.Context, inst *limatype.Instance) string

	BootScripts() (map[string][]byte, error)
}

// GUI defines GUI-related operations.
type GUI interface {
	// RunGUI is for starting GUI synchronously by hostagent. This method should be wait and return only after vm terminates
	// It returns error if there are any failures
	RunGUI() error

	ChangeDisplayPassword(ctx context.Context, password string) error
	DisplayConnection(ctx context.Context) (string, error)
}

// SnapshotManager defines operations for managing snapshots.
type SnapshotManager interface {
	CreateSnapshot(ctx context.Context, tag string) error
	ApplySnapshot(ctx context.Context, tag string) error
	DeleteSnapshot(ctx context.Context, tag string) error
	ListSnapshots(ctx context.Context) (string, error)
}

// GuestAgent defines operations for the guest agent.
type GuestAgent interface {
	// ForwardGuestAgent returns if the guest agent sock needs forwarding by host agent.
	ForwardGuestAgent() bool

	// GuestAgentConn returns the guest agent connection, or nil (if forwarded by ssh).
	GuestAgentConn(_ context.Context) (net.Conn, string, error)
}

// Driver interface is used by hostagent for managing vm.
type Driver interface {
	Lifecycle
	GUI
	SnapshotManager
	GuestAgent

	Info() Info

	// Configure sets the configuration for the instance.
	Configure(inst *limatype.Instance) *ConfiguredDriver

	AcceptConfig(cfg *limatype.LimaYAML, filePath string) error
	FillConfig(cfg *limatype.LimaYAML, filePath string) error

	SSHAddress(ctx context.Context) (string, error)
}

type ConfiguredDriver struct {
	Driver
}

type Info struct {
	VsockPort   int            `json:"vsockPort"`
	VirtioPort  string         `json:"virtioPort"`
	InstanceDir string         `json:"instanceDir,omitempty"`
	Features    DriverFeatures `json:"features"`
}

type DriverFeatures struct {
	DriverName           string `json:"driverName"`
	CanRunGUI            bool   `json:"canRunGui,omitempty"`
	DynamicSSHAddress    bool   `json:"dynamicSSHAddress"`
	SkipSocketForwarding bool   `json:"skipSocketForwarding"`
}
