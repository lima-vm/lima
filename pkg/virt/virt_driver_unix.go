//go:build !windows && !no_virt

package virt

import (
	"context"
	"errors"
	"path/filepath"
	"plugin"

	"github.com/lima-vm/lima/pkg/driver"
	"github.com/lima-vm/lima/pkg/usrlocalliblima"
	"github.com/sirupsen/logrus"
)

const Enabled = true

type LimaVirtDriver struct {
	*driver.BaseDriver

	virtPlugin *plugin.Plugin
}

func New(driver *driver.BaseDriver) *LimaVirtDriver {
	return &LimaVirtDriver{
		BaseDriver: driver,
	}
}

func (l *LimaVirtDriver) Validate() error {
	dir, err := usrlocalliblima.Dir()
	if err != nil {
		return err
	}
	p, err := plugin.Open(filepath.Join(dir, "plugins/virt.so"))
	if err != nil {
		return err
	}

	// test variable
	v, err := p.Lookup("VERSION")
	if err != nil {
		return err
	}
	n := *v.(*uint32)
	logrus.Infof("VERSION: %d", n)
	// test function
	f, err := p.Lookup("Version")
	if err != nil {
		return err
	}
	n, err = f.(func() (uint32, error))()
	if err != nil {
		return err
	}
	logrus.Infof("Version: %d", n)

	l.virtPlugin = p
	return nil
}

func (l *LimaVirtDriver) CreateDisk() error {
	return EnsureDisk(l.BaseDriver)
}

func (l *LimaVirtDriver) Start(_ context.Context) (chan error, error) {
	return nil, errors.New("TODO")
}

func (l *LimaVirtDriver) Stop(_ context.Context) error {
	return nil
}
