/*
Copyright The Lima Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package instance

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/lima-vm/lima/pkg/cidata"
	"github.com/lima-vm/lima/pkg/driver"
	"github.com/lima-vm/lima/pkg/driverutil"
	"github.com/lima-vm/lima/pkg/limayaml"
	"github.com/lima-vm/lima/pkg/osutil"
	"github.com/lima-vm/lima/pkg/store"
	"github.com/lima-vm/lima/pkg/store/filenames"
	"github.com/lima-vm/lima/pkg/version"
)

func Create(ctx context.Context, instName string, instConfig []byte, saveBrokenYAML bool) (*store.Instance, error) {
	if instName == "" {
		return nil, errors.New("got empty instName")
	}
	if len(instConfig) == 0 {
		return nil, errors.New("got empty instConfig")
	}

	instDir, err := store.InstanceDir(instName)
	if err != nil {
		return nil, err
	}

	// the full path of the socket name must be less than UNIX_PATH_MAX chars.
	maxSockName := filepath.Join(instDir, filenames.LongestSock)
	if len(maxSockName) >= osutil.UnixPathMax {
		return nil, fmt.Errorf("instance name %q too long: %q must be less than UNIX_PATH_MAX=%d characters, but is %d",
			instName, maxSockName, osutil.UnixPathMax, len(maxSockName))
	}
	if _, err := os.Stat(instDir); !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("instance %q already exists (%q)", instName, instDir)
	}
	// limayaml.Load() needs to pass the store file path to limayaml.FillDefault() to calculate default MAC addresses
	filePath := filepath.Join(instDir, filenames.LimaYAML)
	loadedInstConfig, err := limayaml.LoadWithWarnings(instConfig, filePath)
	if err != nil {
		return nil, err
	}
	if err := limayaml.Validate(loadedInstConfig, true); err != nil {
		if !saveBrokenYAML {
			return nil, err
		}
		rejectedYAML := "lima.REJECTED.yaml"
		if writeErr := os.WriteFile(rejectedYAML, instConfig, 0o644); writeErr != nil {
			return nil, fmt.Errorf("the YAML is invalid, attempted to save the buffer as %q but failed: %w: %w", rejectedYAML, writeErr, err)
		}
		return nil, fmt.Errorf("the YAML is invalid, saved the buffer as %q: %w", rejectedYAML, err)
	}
	if err := os.MkdirAll(instDir, 0o700); err != nil {
		return nil, err
	}
	if err := os.WriteFile(filePath, instConfig, 0o644); err != nil {
		return nil, err
	}
	if err := cidata.GenerateCloudConfig(instDir, instName, loadedInstConfig); err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(instDir, filenames.LimaVersion), []byte(version.Version), 0o444); err != nil {
		return nil, err
	}

	inst, err := store.Inspect(instName)
	if err != nil {
		return nil, err
	}

	limaDriver := driverutil.CreateTargetDriverInstance(&driver.BaseDriver{
		Instance: inst,
	})

	if err := limaDriver.Register(ctx); err != nil {
		return nil, err
	}

	return inst, nil
}
