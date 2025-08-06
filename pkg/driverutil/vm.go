package driverutil

import (
	"fmt"
	"sort"

	"github.com/lima-vm/lima/v2/pkg/limatype"
	"github.com/lima-vm/lima/v2/pkg/registry"
	"github.com/sirupsen/logrus"
)

func ResolveVMType(y *limatype.LimaYAML, filePath string) error {
	if y.VMType != nil && *y.VMType != "" {
		vmType := *y.VMType
		_, intDriver, exists := registry.Get(vmType)
		if !exists {
			return fmt.Errorf("specified vmType %q is not a registered driver", vmType)
		}
		if intDriver == nil {
			// For now we only support internal drivers.
			return fmt.Errorf("specified vmType %q is not an internal driver", vmType)
		}
		if err := intDriver.AcceptConfig(y, filePath); err != nil {
			return fmt.Errorf("vmType %q is not compatible with the configuration: %w", vmType, err)
		}
		if err := intDriver.FillConfig(y, filePath); err != nil {
			return fmt.Errorf("unable to fill config for vmType %q: %w", vmType, err)
		}
		logrus.Debugf("ResolveVMType: using explicitly specified VMType %q", vmType)
		return nil
	}

	// If VMType is not specified, we try to resolve it by checking config with all the registered drivers.
	candidates := registry.List()
	vmtypes := make([]string, 0, len(candidates))
	for vmtype := range candidates {
		vmtypes = append(vmtypes, vmtype)
	}
	sort.Strings(vmtypes)

	for _, vmType := range vmtypes {
		// For now we only support internal drivers.
		if registry.CheckInternalOrExternal(vmType) == registry.Internal {
			_, intDriver, _ := registry.Get(vmType)
			if err := intDriver.AcceptConfig(y, filePath); err == nil {
				logrus.Debugf("ResolveVMType: resolved VMType %q", vmType)
				if err := intDriver.FillConfig(y, filePath); err != nil {
					return fmt.Errorf("unable to fill config for VMType %q: %w", vmType, err)
				}
				return nil
			}
		}
	}

	return fmt.Errorf("no VMType found for %q", filePath)
}
