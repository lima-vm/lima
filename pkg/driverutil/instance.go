// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package driverutil

import (
	"fmt"

	"github.com/sirupsen/logrus"

	"github.com/lima-vm/lima/v2/pkg/driver"
	"github.com/lima-vm/lima/v2/pkg/driver/external/server"
	"github.com/lima-vm/lima/v2/pkg/registry"
	"github.com/lima-vm/lima/v2/pkg/store"
)

// CreateConfiguredDriver creates a driver.ConfiguredDriver for the given instance.
func CreateConfiguredDriver(inst *store.Instance, sshLocalPort int) (*driver.ConfiguredDriver, error) {
	limaDriver := inst.Config.VMType
	extDriver, intDriver, exists := registry.Get(*limaDriver)
	if !exists {
		return nil, fmt.Errorf("unknown or unsupported VM type: %s", *limaDriver)
	}

	if extDriver != nil {
		extDriver.Logger.Debugf("Using external driver %q", extDriver.Name)
		if extDriver.Client == nil || extDriver.Command == nil {
			logrus.Debugf("Starting new instance of external driver %q", extDriver.Name)
			if err := server.Start(extDriver, inst.Name); err != nil {
				extDriver.Logger.Errorf("Failed to start external driver %q: %v", extDriver.Name, err)
				return nil, err
			}
		} else {
			logrus.Debugf("Reusing existing external driver %q instance", extDriver.Name)
			extDriver.InstanceName = inst.Name
		}

		return extDriver.Client.Configure(inst, sshLocalPort), nil
	}

	logrus.Debugf("Using internal driver %q", intDriver.Info().DriverName)
	return intDriver.Configure(inst, sshLocalPort), nil
}
