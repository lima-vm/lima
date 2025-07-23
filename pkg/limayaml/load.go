// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package limayaml

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"

	"github.com/lima-vm/lima/v2/pkg/limatype"
	"github.com/lima-vm/lima/v2/pkg/ptr"
	"github.com/lima-vm/lima/v2/pkg/registry"
	"github.com/lima-vm/lima/v2/pkg/store/dirnames"
	"github.com/lima-vm/lima/v2/pkg/store/filenames"
)

// Load loads the yaml and fulfills unspecified fields with the default values.
//
// Load does not validate. Use Validate for validation.
func Load(ctx context.Context, b []byte, filePath string) (*limatype.LimaYAML, error) {
	return load(ctx, b, filePath, false)
}

// LoadWithWarnings will call FillDefaults with warnings enabled (e.g. when
// the username is not valid on Linux and must be replaced by "Lima").
// It is called when creating or editing an instance.
func LoadWithWarnings(ctx context.Context, b []byte, filePath string) (*limatype.LimaYAML, error) {
	return load(ctx, b, filePath, true)
}

func load(ctx context.Context, b []byte, filePath string, warn bool) (*limatype.LimaYAML, error) {
	var y, d, o limatype.LimaYAML

	if err := Unmarshal(b, &y, fmt.Sprintf("main file %q", filePath)); err != nil {
		return nil, err
	}
	configDir, err := dirnames.LimaConfigDir()
	if err != nil {
		return nil, err
	}

	defaultPath := filepath.Join(configDir, filenames.Default)
	bytes, err := os.ReadFile(defaultPath)
	if err == nil {
		logrus.Debugf("Mixing %q into %q", defaultPath, filePath)
		if err := Unmarshal(bytes, &d, fmt.Sprintf("default file %q", defaultPath)); err != nil {
			return nil, err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}

	overridePath := filepath.Join(configDir, filenames.Override)
	bytes, err = os.ReadFile(overridePath)
	if err == nil {
		logrus.Debugf("Mixing %q into %q", overridePath, filePath)
		if err := Unmarshal(bytes, &o, fmt.Sprintf("override file %q", overridePath)); err != nil {
			return nil, err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}

	// It should be called before the `y` parameter is passed to FillDefault() that execute template.
	if err := validateParamIsUsed(&y); err != nil {
		return nil, err
	}

	FillDefault(ctx, &y, &d, &o, filePath, warn)
	vmType, err := ResolveVMType(&y, filePath)
	if err != nil {
		logrus.WithError(err).Warnf("Failed to resolve VMType for %q", filePath)
	}
	y.VMType = ptr.Of(vmType)

	return &y, nil
}

func ResolveVMType(y *limatype.LimaYAML, filePath string) (limatype.VMType, error) {
	// Check if the VMType is explicitly specified
	if y.VMType != nil && *y.VMType != "" && *y.VMType != "default" {
		logrus.Debugf("ResolveVMType: VMType %q is explicitly specified in %q", *y.VMType, filePath)
		_, _, exists := registry.Get(*y.VMType)
		if !exists {
			logrus.Debugf("ResolveVMType: VMType %q is not registered, using default VMType %q", *y.VMType, limatype.QEMU)
			return "", fmt.Errorf("VMType %q is not registered", *y.VMType)
		}
		return limatype.NewVMType(*y.VMType), nil
	}

	candidates := registry.List()
	for vmType, location := range candidates {
		if location != registry.External {
			// For now we only support internal drivers.
			_, intDriver, _ := registry.Get(vmType)
			if err := intDriver.AcceptConfig(y, filePath); err == nil {
				logrus.Debugf("ResolveVMType: resolved VMType %q (from %q)", vmType, location)
				return limatype.NewVMType(vmType), nil
			} else {
				logrus.Debugf("ResolveVMType: VMType %q is not accepted by the driver: %v", vmType, err)
			}
		}
	}

	return "", errors.New("no driver can handle this config")
}
