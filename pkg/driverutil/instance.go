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
func CreateTargetDriverInstance(inst *store.Instance, sshLocalPort int) (driver.Driver, error) {
	limaDriver := inst.Config.VMType
	driver, exists := registry.DefaultRegistry.Get(string(*limaDriver), inst.Name)
	if !exists {
		return nil, fmt.Errorf("unknown or unsupported VM type: %s", *limaDriver)
	}

	driver.SetConfig(inst, sshLocalPort)

	return driver, nil
}
