/*
Copyright The Lima Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package driver

import (
	"context"
	"errors"
	"net"

	"github.com/lima-vm/lima/pkg/store"
)

// Driver interface is used by hostagent for managing vm.
//
// This interface is extended by BaseDriver which provides default implementation.
// All other driver definition must extend BaseDriver.
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
}

type BaseDriver struct {
	Instance *store.Instance

	SSHLocalPort int
	VSockPort    int
	VirtioPort   string
}

var _ Driver = (*BaseDriver)(nil)

func (d *BaseDriver) Validate() error {
	return nil
}

func (d *BaseDriver) Initialize(_ context.Context) error {
	return nil
}

func (d *BaseDriver) CreateDisk(_ context.Context) error {
	return nil
}

func (d *BaseDriver) Start(_ context.Context) (chan error, error) {
	return nil, nil
}

func (d *BaseDriver) CanRunGUI() bool {
	return false
}

func (d *BaseDriver) RunGUI() error {
	return nil
}

func (d *BaseDriver) Stop(_ context.Context) error {
	return nil
}

func (d *BaseDriver) Register(_ context.Context) error {
	return nil
}

func (d *BaseDriver) Unregister(_ context.Context) error {
	return nil
}

func (d *BaseDriver) ChangeDisplayPassword(_ context.Context, _ string) error {
	return nil
}

func (d *BaseDriver) GetDisplayConnection(_ context.Context) (string, error) {
	return "", nil
}

func (d *BaseDriver) CreateSnapshot(_ context.Context, _ string) error {
	return errors.New("unimplemented")
}

func (d *BaseDriver) ApplySnapshot(_ context.Context, _ string) error {
	return errors.New("unimplemented")
}

func (d *BaseDriver) DeleteSnapshot(_ context.Context, _ string) error {
	return errors.New("unimplemented")
}

func (d *BaseDriver) ListSnapshots(_ context.Context) (string, error) {
	return "", errors.New("unimplemented")
}

func (d *BaseDriver) ForwardGuestAgent() bool {
	// if driver is not providing, use host agent
	return d.VSockPort == 0 && d.VirtioPort == ""
}

func (d *BaseDriver) GuestAgentConn(_ context.Context) (net.Conn, error) {
	// use the unix socket forwarded by host agent
	return nil, nil
}
