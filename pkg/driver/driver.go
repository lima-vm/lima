package driver

import (
	"context"

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
