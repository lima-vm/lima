package driverutil

import (
	"github.com/lima-vm/lima/pkg/driver"
	"github.com/lima-vm/lima/pkg/limayaml"
	"github.com/lima-vm/lima/pkg/qemu"
	"github.com/lima-vm/lima/pkg/vz"
)

func CreateTargetDriverInstance(base *driver.BaseDriver) driver.Driver {
	limaDriver := base.Yaml.VMType
	if *limaDriver == limayaml.VZ {
		return vz.New(base)
	}
	return qemu.New(base)
}
