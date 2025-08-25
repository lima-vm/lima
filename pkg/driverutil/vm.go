// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package driverutil

import (
	"context"
	"errors"
	"fmt"

	"github.com/sirupsen/logrus"

	"github.com/lima-vm/lima/v2/pkg/limatype"
	"github.com/lima-vm/lima/v2/pkg/registry"
)

func ResolveVMType(y *limatype.LimaYAML, filePath string) error {
	if y.VMType != nil && *y.VMType != "" {
		if err := validateConfigAgainstDriver(y, filePath, *y.VMType); err != nil {
			return err
		}
		logrus.Debugf("Using specified vmType %q for %q", *y.VMType, filePath)
		return nil
	}

	// If VMType is not specified, we go with the default platform driver.
	vmType := limatype.DefaultDriver()
	return validateConfigAgainstDriver(y, filePath, vmType)
}

func validateConfigAgainstDriver(y *limatype.LimaYAML, filePath, vmType string) error {
	extDriver, intDriver, exists := registry.Get(vmType)
	if !exists {
		return fmt.Errorf("vmType %q is not a registered driver", vmType)
	}

	if extDriver != nil {
		return errors.New("not supported for external drivers")
	}

	if err := intDriver.AcceptConfig(y, filePath); err != nil {
		return err
	}
	if err := intDriver.FillConfig(y, filePath); err != nil {
		return err
	}

	return nil
}

func InspectStatus(ctx context.Context, inst *limatype.Instance) (string, error) {
	if inst == nil || inst.Config == nil || inst.Config.VMType == nil {
		return "", errors.New("instance or its configuration is not properly initialized")
	}

	extDriver, intDriver, exists := registry.Get(*inst.Config.VMType)
	if !exists {
		return "", fmt.Errorf("unknown or unsupported VM type: %s", *inst.Config.VMType)
	}

	if extDriver != nil {
		return "", errors.New("InspectStatus is not supported for external drivers")
	}

	return intDriver.InspectStatus(ctx, inst), nil
}
