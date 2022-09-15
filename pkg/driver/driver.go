package driver

import (
	"context"
	"fmt"

	"github.com/lima-vm/lima/pkg/limayaml"
	"github.com/lima-vm/lima/pkg/store"
)

type Driver interface {
	Validate() error

	CreateDisk() error

	Start(_ context.Context) (chan error, error)

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

func (d *BaseDriver) Stop(_ context.Context) error {
	return nil
}

func (d *BaseDriver) ChangeDisplayPassword(_ context.Context, password string) error {
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
