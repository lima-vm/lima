// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package driverutil

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"slices"

	"github.com/sirupsen/logrus"

	"github.com/lima-vm/lima/v2/pkg/limatype"
	"github.com/lima-vm/lima/v2/pkg/ptr"
	"github.com/lima-vm/lima/v2/pkg/registry"
)

// ResolveVMType sets y.VMType to the appropriate default if it is nil.
// It validates the VMType is a known type (built-in or discovered external driver)
// but does NOT require the driver to be available on the current platform.
func ResolveVMType(y *limatype.LimaYAML) error {
	if y.VMType == nil {
		y.VMType = ptr.Of(limatype.DefaultDriver())
		if y.Arch != nil && !limatype.IsNativeArch(*y.Arch) {
			y.VMType = ptr.Of(limatype.DefaultNonNativeArchDriver())
		}
	}

	// Accept built-in VMType regardless of current platform.
	if slices.Contains(limatype.VMTypes, *y.VMType) {
		return nil
	}

	// Also accept external drivers discovered in the registry.
	_, _, exists := registry.Get(*y.VMType)
	if !exists {
		return fmt.Errorf("vmType %#q is not a registered driver", *y.VMType)
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
		status, err := handleInspectStatusAction(ctx, inst, extDriver.Path)
		if err != nil {
			extDriver.Logger.Errorf("Failed to inspect status for instance %#q: %v", inst.Name, err)
			return "", err
		}
		extDriver.Logger.Debugf("Instance %#q inspected successfully with status: %s", inst.Name, inst.Status)
		return status, nil
	}

	return intDriver.InspectStatus(ctx, inst), nil
}

func handleInspectStatusAction(ctx context.Context, inst *limatype.Instance, extDriverPath string) (string, error) {
	cmd := exec.CommandContext(ctx, extDriverPath, "--inspect-status")

	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return "", err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", err
	}
	if err := cmd.Start(); err != nil {
		return "", err
	}

	encoder := json.NewEncoder(stdin)
	payload, err := inst.MarshalJSON()
	if err != nil {
		return "", fmt.Errorf("failed to marshal instance config: %w", err)
	}
	if err := encoder.Encode(payload); err != nil {
		return "", err
	}
	stdin.Close()

	decoder := json.NewDecoder(stdout)
	var response []byte
	if err := decoder.Decode(&response); err != nil {
		return "", err
	}

	var respInst limatype.Instance
	if err := respInst.UnmarshalJSON(response); err != nil {
		return "", fmt.Errorf("failed to unmarshal instance response: %w", err)
	}

	if err := cmd.Wait(); err != nil {
		if stderrBuf.Len() > 0 {
			return "", fmt.Errorf("inspect status command failed: %w; stderr: %s", err, stderrBuf.String())
		}
		return "", fmt.Errorf("inspect status command failed: %w", err)
	}

	if stderrBuf.Len() > 0 {
		logrus.Debugf("external driver stderr: %s", stderrBuf.String())
	}

	*inst = respInst
	logrus.Debugf("Inspecting instance status action completed successfully for %#q", extDriverPath)
	return inst.Status, nil
}
