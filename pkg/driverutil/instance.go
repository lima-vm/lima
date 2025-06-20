// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package driverutil

import (
	"fmt"

	"github.com/lima-vm/lima/pkg/driver"
	"github.com/lima-vm/lima/pkg/registry"
	"github.com/lima-vm/lima/pkg/store"
)

// CreateTargetDriverInstance creates the appropriate driver for an instance.
func CreateConfiguredDriver(inst *store.Instance, sshLocalPort int) (driver.ConfiguredDriver, error) {
	limaDriver := inst.Config.VMType
	driver, exists := registry.Get(string(*limaDriver), inst.Name)
	if !exists {
		return driver.Configure(nil, 0), fmt.Errorf("unknown or unsupported VM type: %s", *limaDriver)
	}

	return driver.Configure(inst, sshLocalPort), nil
}
