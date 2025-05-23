// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package driverutil

import (
	"fmt"

	"github.com/lima-vm/lima/pkg/driver"
	"github.com/lima-vm/lima/pkg/registry"
	"github.com/lima-vm/lima/pkg/store"
)

// CreateTargetDriverInstance creates the appropriate driver for an instance
func CreateTargetDriverInstance(inst *store.Instance, sshLocalPort int) (driver.Driver, error) {
	limaDriver := inst.Config.VMType
	driver, exists := registry.DefaultRegistry.Get(string(*limaDriver))
	if !exists {
		return nil, fmt.Errorf("unknown or unsupported VM type: %s", *limaDriver)
	}

	if err := driver.Validate(); err != nil {
		return nil, fmt.Errorf("driver validation failed: %w", err)
	}

	return driver.NewDriver(inst, sshLocalPort), nil
}
