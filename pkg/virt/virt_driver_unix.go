//go:build !windows && !no_virt

package virt

import (
	"context"
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

func (l *LimaVirtDriver) loadPlugin() error {
	dir, err := usrlocalliblima.Dir()
	if err != nil {
		return err
	}
	p, err := plugin.Open(filepath.Join(dir, "plugins/virt.so"))
	if err != nil {
		return err
	}
	l.virtPlugin = p
	return nil
}

func (l *LimaVirtDriver) Validate() error {
	err := l.loadPlugin()
	if err != nil {
		return err
	}
	p := l.virtPlugin
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

	return nil
}

func (l *LimaVirtDriver) CreateDisk() error {
	return EnsureDisk(l.BaseDriver)
}

func (l *LimaVirtDriver) Start(_ context.Context) (chan error, error) {
	err := l.loadPlugin()
	if err != nil {
		return nil, err
	}

	/*
	net, err := NetworkXML(l.BaseDriver)
	if err != nil {
		return nil, err
	}
	cn, err := l.virtPlugin.Lookup("CreateNetwork")
	if err != nil {
		return nil, err
	}
	if err := cn.(func(string) error)(net); err != nil {
		return nil, err
	}
	*/

	dom, err := DomainXML(l.BaseDriver)
	if err != nil {
		return nil, err
	}
	cd, err := l.virtPlugin.Lookup("CreateDomain")
	if err != nil {
		return nil, err
	}
	if err := cd.(func(string) error)(dom); err != nil {
		return nil, err
	}

	return nil, err
}

func (l *LimaVirtDriver) Stop(_ context.Context) error {
	return nil
}
