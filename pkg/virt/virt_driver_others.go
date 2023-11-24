//go:build windows || no_virt

package virt

import (
	"context"
	"errors"

	"github.com/lima-vm/lima/pkg/driver"
)

var ErrUnsupported = errors.New("vm driver 'virt' is not supported")

const Enabled = false

type LimaVirtDriver struct {
	*driver.BaseDriver
}

func New(driver *driver.BaseDriver) *LimaVirtDriver {
	return &LimaVirtDriver{
		BaseDriver: driver,
	}
}

func (l *LimaVirtDriver) Validate() error {
	return ErrUnsupported
}

func (l *LimaVirtDriver) CreateDisk() error {
	return ErrUnsupported
}

func (l *LimaVirtDriver) Start(_ context.Context) (chan error, error) {
	return nil, ErrUnsupported
}

func (l *LimaVirtDriver) Stop(_ context.Context) error {
	return ErrUnsupported
}
