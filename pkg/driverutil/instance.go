// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package driverutil

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"

	"github.com/lima-vm/lima/v2/pkg/driver"
	"github.com/lima-vm/lima/v2/pkg/driver/external/server"
	"github.com/lima-vm/lima/v2/pkg/limatype"
	"github.com/lima-vm/lima/v2/pkg/registry"
)

// CreateConfiguredDriver creates a driver.ConfiguredDriver for the given instance.
// For external drivers, it reuses an existing server if one is already running,
// or starts a new one if needed.
func CreateConfiguredDriver(ctx context.Context, inst *limatype.Instance, sshLocalPort int) (*driver.ConfiguredDriver, error) {
	limaDriver := inst.Config.VMType
	extDriver, intDriver, exists := registry.Get(*limaDriver)
	if !exists {
		return nil, fmt.Errorf("unknown or unsupported VM type: %s", *limaDriver)
	}
	var driverInfo driver.Info
	if extDriver != nil {
		extDriver.Logger.Debugf("Connecting to external driver %#q for %#q", extDriver.Name, inst.Name)
		if err := server.Start(ctx, extDriver, inst.Name); err != nil {
			extDriver.Logger.Errorf("Failed to start external driver %#q: %v", extDriver.Name, err)
			return nil, err
		}

		driverInfo = extDriver.Client.Info(ctx)
		if !driverInfo.Features.StaticSSHPort {
			inst.SSHLocalPort = sshLocalPort
		}
		return extDriver.Client.Configure(ctx, inst)
	}

	driverInfo = intDriver.Info(ctx)
	logrus.Debugf("Using internal driver %q", driverInfo.Name)
	if !driverInfo.Features.StaticSSHPort {
		inst.SSHLocalPort = sshLocalPort
	}
	return intDriver.Configure(ctx, inst)
}
