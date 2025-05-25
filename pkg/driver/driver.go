// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package driver

import (
	"context"
	"net"

	"github.com/lima-vm/lima/pkg/store"
)

// Lifecycle defines basic lifecycle operations.
type Lifecycle interface {
	// Validate returns error if the current driver isn't support for given config
	Validate() error

	// Initialize is called on creating the instance for initialization.
	// (e.g., creating "vz-identifier" file)
	//
	// Initialize MUST return nil when it is called against an existing instance.
	//
	// Initialize does not create the disks.
	Initialize(_ context.Context) error

	// CreateDisk returns error if the current driver fails in creating disk
	CreateDisk(_ context.Context) error

	// Start is used for booting the vm using driver instance
	// It returns a chan error on successful boot
	// The second argument may contain error occurred while starting driver
	Start(_ context.Context) (chan error, error)

	// Stop will terminate the running vm instance.
	// It returns error if there are any errors during Stop
	Stop(_ context.Context) error
}

// GUI defines GUI-related operations.
type GUI interface {
	// CanRunGUI returns bool to indicate if the hostagent need to run GUI synchronously
	CanRunGUI() bool

	// RunGUI is for starting GUI synchronously by hostagent. This method should be wait and return only after vm terminates
	// It returns error if there are any failures
	RunGUI() error

	ChangeDisplayPassword(ctx context.Context, password string) error
	GetDisplayConnection(ctx context.Context) (string, error)
}

// Snapshot defines operations for managing snapshots.
type Snapshot interface {
	CreateSnapshot(ctx context.Context, tag string) error
	ApplySnapshot(ctx context.Context, tag string) error
	DeleteSnapshot(ctx context.Context, tag string) error
	ListSnapshots(ctx context.Context) (string, error)
}

// Registration defines operations for registering and unregistering the driver instance.
type Registration interface {
	Register(ctx context.Context) error
	Unregister(ctx context.Context) error
}

// GuestAgent defines operations for the guest agent.
type GuestAgent interface {
	// ForwardGuestAgent returns if the guest agent sock needs forwarding by host agent.
	ForwardGuestAgent() bool

	// GuestAgentConn returns the guest agent connection, or nil (if forwarded by ssh).
	GuestAgentConn(_ context.Context) (net.Conn, error)
}

type Plugin interface {
	// Name returns the name of the driver
	Name() string

	// SetConfig sets the configuration for the instance.
	SetConfig(inst *store.Instance, sshLocalPort int)
}

// Driver interface is used by hostagent for managing vm.
type Driver interface {
	Lifecycle
	GUI
	Snapshot
	Registration
	GuestAgent
	Plugin

	GetVSockPort() int
	GetVirtioPort() string
}
