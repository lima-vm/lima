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

const (
	OwnerHostAgent = "ha"
	OwnerCLI       = "cli"
)

// CreateConfiguredDriver creates a driver.ConfiguredDriver for the given instance.
// pidFileOwner identifies the caller (e.g. "ha" or "cli") to isolate driver PID files.
func CreateConfiguredDriver(ctx context.Context, inst *limatype.Instance, sshLocalPort int, pidFileOwner string) (*driver.ConfiguredDriver, error) {
	limaDriver := inst.Config.VMType
	extDriver, intDriver, exists := registry.Get(*limaDriver)
	if !exists {
		return nil, fmt.Errorf("unknown or unsupported VM type: %s", *limaDriver)
	}
	if pidFileOwner == "" {
		pidFileOwner = OwnerCLI
	}
	if extDriver != nil {
		extDriver.PIDFileOwner = pidFileOwner
		extDriver.Logger.Debugf("Using external driver %#q", extDriver.Name)
		if extDriver.Client == nil || extDriver.Command == nil {
			logrus.Debugf("Starting new instance of external driver %#q", extDriver.Name)
			if err := server.Start(ctx, extDriver, inst.Name); err != nil {
				extDriver.Logger.Errorf("Failed to start external driver %#q: %v", extDriver.Name, err)
				return nil, err
			}
		} else {
			logrus.Debugf("Reusing existing external driver %#q instance", extDriver.Name)
			extDriver.InstanceName = inst.Name
		}

		info := extDriver.Client.Info(ctx)
		if !info.Features.StaticSSHPort {
			inst.SSHLocalPort = sshLocalPort
		}
		return extDriver.Client.Configure(ctx, inst), nil
	}

	info := intDriver.Info(ctx)
	logrus.Debugf("Using internal driver %#q", info.Name)
	if !info.Features.StaticSSHPort {
		inst.SSHLocalPort = sshLocalPort
	}
	return intDriver.Configure(ctx, inst), nil
}
