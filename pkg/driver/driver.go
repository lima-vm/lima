package driver

import (
	"context"
	"fmt"

	"github.com/lima-vm/lima/pkg/limayaml"
	"github.com/lima-vm/lima/pkg/store"
)

// Driver interface is used by hostagent for managing vm.
//
// This interface is extended by BaseDriver which provides default implementation.
// All other driver definition must extend BaseDriver
type Driver interface {
	// Validate returns error if the current driver isn't support for given config
	Validate() error

	// CreateDisk returns error if the current driver fails in creating disk
	CreateDisk() error

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

	ChangeDisplayPassword(_ context.Context, password string) error

	GetDisplayConnection(_ context.Context) (string, error)

	CreateSnapshot(_ context.Context, tag string) error

	ApplySnapshot(_ context.Context, tag string) error

	DeleteSnapshot(_ context.Context, tag string) error

	ListSnapshots(_ context.Context) (string, error)
}

type BaseDriver struct {
	Instance *store.Instance
	Yaml     *limayaml.LimaYAML

	SSHLocalPort int
}

func (d *BaseDriver) Validate() error {
	return nil
}

func (d *BaseDriver) CreateDisk() error {
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

func (d *BaseDriver) ChangeDisplayPassword(_ context.Context, _ string) error {
	return nil
}

func (d *BaseDriver) GetDisplayConnection(_ context.Context) (string, error) {
	return "", nil
}

func (d *BaseDriver) CreateSnapshot(_ context.Context, _ string) error {
	return fmt.Errorf("unimplemented")
}

func (d *BaseDriver) ApplySnapshot(_ context.Context, _ string) error {
	return fmt.Errorf("unimplemented")
}

func (d *BaseDriver) DeleteSnapshot(_ context.Context, _ string) error {
	return fmt.Errorf("unimplemented")
}

func (d *BaseDriver) ListSnapshots(_ context.Context) (string, error) {
	return "", fmt.Errorf("unimplemented")
}
