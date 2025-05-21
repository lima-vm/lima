// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package driverutil

import (
	"fmt"

	"github.com/sirupsen/logrus"

	"github.com/lima-vm/lima/pkg/driver"
	"github.com/lima-vm/lima/pkg/registry"
	"github.com/lima-vm/lima/pkg/store"
)

// CreateConfiguredDriver creates a driver.ConfiguredDriver for the given instance.
func CreateConfiguredDriver(inst *store.Instance, sshLocalPort int) (*driver.ConfiguredDriver, error) {
	limaDriver := inst.Config.VMType
	_, intDriver, exists := registry.Get(*limaDriver)
	if !exists {
		return nil, fmt.Errorf("unknown or unsupported VM type: %s", *limaDriver)
	}

	logrus.Infof("Using internal driver %q", intDriver.Info().DriverName)
	return intDriver.Configure(inst, sshLocalPort), nil
}
