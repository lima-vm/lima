package virt

import (
	"context"
	"os/exec"

	"github.com/lima-vm/lima/pkg/driver"
	"github.com/sirupsen/logrus"
)

const Enabled = true

type LimaVirtDriver struct {
	*driver.BaseDriver
}

func New(driver *driver.BaseDriver) *LimaVirtDriver {
	return &LimaVirtDriver{
		BaseDriver: driver,
	}
}

func (l *LimaVirtDriver) Validate() error {
	if _, err := exec.LookPath("virsh"); err != nil {
		return err
	}

	v, err := Version()
	if err != nil {
		return err
	}
	logrus.Infof("Version: %s", v)

	return nil
}

func (l *LimaVirtDriver) CreateDisk() error {
	return EnsureDisk(l.BaseDriver)
}

func (l *LimaVirtDriver) Start(_ context.Context) (chan error, error) {
	/*
		net, err := NetworkXML(l.BaseDriver)
		if err != nil {
			return nil, err
		}
		if err := CreateNetwork(net); err != nil {
			return nil, err
		}
	*/

	dom, err := DomainXML(l.BaseDriver)
	if err != nil {
		return nil, err
	}
	if err := CreateDomain(dom); err != nil {
		return nil, err
	}

	return nil, err
}

func (l *LimaVirtDriver) Stop(_ context.Context) error {
	return nil
}
