package driverutil

import (
	"fmt"

	"github.com/lima-vm/lima/v2/pkg/limatype"
	"github.com/lima-vm/lima/v2/pkg/registry"
	"github.com/sirupsen/logrus"
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
	if err := validateConfigAgainstDriver(y, filePath, vmType); err == nil {
		return nil
	} else {
		return err
	}
}

func validateConfigAgainstDriver(y *limatype.LimaYAML, filePath, vmType string) error {
	_, intDriver, exists := registry.Get(vmType)
	if !exists {
		return fmt.Errorf("vmType %q is not a registered driver", vmType)
	}
	// For now we only support internal drivers.
	if intDriver == nil {
		return fmt.Errorf("vmType %q is not an internal driver", vmType)
	}
	if err := intDriver.AcceptConfig(y, filePath); err != nil {
		return err
	}
	if err := intDriver.FillConfig(y, filePath); err != nil {
		return err
	}

	return nil
}
