// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package driver

import (
	"context"
	"net"
)

// Driver interface is used by hostagent for managing vm.
type Driver interface {
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

	// CanRunGUI returns bool to indicate if the hostagent need to run GUI synchronously
	CanRunGUI() bool

	// RunGUI is for starting GUI synchronously by hostagent. This method should be wait and return only after vm terminates
	// It returns error if there are any failures
	RunGUI() error

	// Stop will terminate the running vm instance.
	// It returns error if there are any errors during Stop
	Stop(_ context.Context) error

	// Register will add an instance to a registry.
	// It returns error if there are any errors during Register
	Register(_ context.Context) error

	// Unregister will perform any cleanup related to the vm instance.
	// It returns error if there are any errors during Unregister
	Unregister(_ context.Context) error

	ChangeDisplayPassword(_ context.Context, password string) error

	GetDisplayConnection(_ context.Context) (string, error)

	CreateSnapshot(_ context.Context, tag string) error

	ApplySnapshot(_ context.Context, tag string) error

	DeleteSnapshot(_ context.Context, tag string) error

	ListSnapshots(_ context.Context) (string, error)

	// ForwardGuestAgent returns if the guest agent sock needs forwarding by host agent.
	ForwardGuestAgent() bool

	// GuestAgentConn returns the guest agent connection, or nil (if forwarded by ssh).
	GuestAgentConn(_ context.Context) (net.Conn, error)

	// Returns the driver name.
	Name() string
}
