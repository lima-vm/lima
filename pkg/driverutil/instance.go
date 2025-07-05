// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package driverutil

import (
	"fmt"

	"github.com/lima-vm/lima/pkg/driver"
	"github.com/lima-vm/lima/pkg/driver/external/server"
	"github.com/lima-vm/lima/pkg/registry"
	"github.com/lima-vm/lima/pkg/store"
	"github.com/sirupsen/logrus"
)

// CreateTargetDriverInstance creates the appropriate driver for an instance.
func CreateConfiguredDriver(inst *store.Instance, sshLocalPort int) (*driver.ConfiguredDriver, error) {
	limaDriver := inst.Config.VMType
	extDriver, intDriver, exists := registry.Get(string(*limaDriver))
	if !exists {
		return nil, fmt.Errorf("unknown or unsupported VM type: %s", *limaDriver)
	}

	if extDriver != nil {
		extDriver.Logger.Debugf("Using external driver %q", extDriver.Name)
		if extDriver.Client == nil || extDriver.Command == nil || extDriver.Command.Process == nil {
			logrus.Infof("Starting new instance of external driver %q", extDriver.Name)
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

	logrus.Infof("Using internal driver %q", intDriver.Info().DriverName)
	return intDriver.Configure(inst, sshLocalPort), nil
}
