// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package driverutil

import (
	"github.com/lima-vm/lima/pkg/driver/vz"
	"github.com/lima-vm/lima/pkg/driver/wsl2"
	"github.com/lima-vm/lima/pkg/limayaml"
)

// Drivers returns the available drivers.
func Drivers() []string {
	drivers := []string{limayaml.QEMU}
	if vz.Enabled {
		drivers = append(drivers, limayaml.VZ)
	}
	if wsl2.Enabled {
		drivers = append(drivers, limayaml.WSL2)
	}
	return drivers
}
