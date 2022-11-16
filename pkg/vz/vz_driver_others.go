//go:build !darwin
// +build !darwin

package vz

import (
	"context"
	"fmt"

	"github.com/lima-vm/lima/pkg/driver"
)

type LimaVzDriver struct {
	*driver.BaseDriver
}

func New(driver *driver.BaseDriver) *LimaVzDriver {
	return &LimaVzDriver{
		BaseDriver: driver,
	}
}

func (l *LimaVzDriver) Validate() error {
	return fmt.Errorf("driver 'vz' is only supported on darwin")
}

func (l *LimaVzDriver) CreateDisk() error {
	return fmt.Errorf("driver 'vz' is only supported on darwin")
}

func (l *LimaVzDriver) Start(ctx context.Context) (chan error, error) {
	return nil, fmt.Errorf("driver 'vz' is only supported on darwin")
}

func (l *LimaVzDriver) Stop(_ context.Context) error {
	return fmt.Errorf("driver 'vz' is only supported on darwin")
}
