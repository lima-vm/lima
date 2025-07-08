// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package driverutil

import (
	"github.com/lima-vm/lima/pkg/driver"
	"github.com/lima-vm/lima/pkg/driver/qemu"
	"github.com/lima-vm/lima/pkg/driver/vz"
	"github.com/lima-vm/lima/pkg/driver/wsl2"
	"github.com/lima-vm/lima/pkg/limayaml"
	"github.com/lima-vm/lima/pkg/store"
)

func CreateTargetDriverInstance(inst *store.Instance, sshLocalPort int) driver.Driver {
	limaDriver := inst.Config.VMType
	if *limaDriver == limayaml.VZ {
		return vz.New(inst, sshLocalPort)
	}
	if *limaDriver == limayaml.WSL2 {
		return wsl2.New(inst, sshLocalPort)
	}
	return qemu.New(inst, sshLocalPort)
}
