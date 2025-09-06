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

	"github.com/sirupsen/logrus"

	"github.com/lima-vm/lima/v2/pkg/limatype"
	"github.com/lima-vm/lima/v2/pkg/registry"
)

// ResolveVMType sets the VMType field in the given LimaYAML if not already set.
// It validates the configuration against the specified or default VMType.
func ResolveVMType(ctx context.Context, y *limatype.LimaYAML, filePath string) error {
	if y.VMType != nil && *y.VMType != "" {
		if err := validateConfigAgainstDriver(ctx, y, filePath, *y.VMType); err != nil {
			return err
		}
		logrus.Debugf("Using specified vmType %q for %q", *y.VMType, filePath)
		return nil
	}

	// If VMType is not specified, we go with the default platform driver.
	vmType := limatype.DefaultDriver()
	return validateConfigAgainstDriver(ctx, y, filePath, vmType)
}

func validateConfigAgainstDriver(ctx context.Context, y *limatype.LimaYAML, filePath, vmType string) error {
	extDriver, intDriver, exists := registry.Get(vmType)
	if !exists {
		return fmt.Errorf("vmType %q is not a registered driver", vmType)
	}

	if extDriver != nil {
		return handlePreConfiguredDriverAction(ctx, y, extDriver.Path, filePath)
	}

	if err := intDriver.FillConfig(ctx, y, filePath); err != nil {
		return err
	}

	return nil
}

func handlePreConfiguredDriverAction(ctx context.Context, y *limatype.LimaYAML, extDriverPath, filePath string) error {
	cmd := exec.CommandContext(ctx, extDriverPath, "--pre-driver-action")

	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdout pipe: %w", err)
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdin pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start external driver: %w", err)
	}

	encoder := json.NewEncoder(stdin)
	if err := encoder.Encode(limatype.PreConfiguredDriverPayload{
		Config:   *y,
		FilePath: filePath,
	}); err != nil {
		stdin.Close()
		return fmt.Errorf("failed to encode pre-configured driver payload: %w", err)
	}
	stdin.Close()

	decoder := json.NewDecoder(stdout)
	var res limatype.LimaYAML
	if err := decoder.Decode(&res); err != nil {
		return fmt.Errorf("failed to decode pre-configured driver response: %w", err)
	}

	if err := cmd.Wait(); err != nil {
		if stderrBuf.Len() > 0 {
			return fmt.Errorf("pre-configured driver command failed: %w; stderr: %s", err, stderrBuf.String())
		}
		return fmt.Errorf("pre-configured driver command failed: %w", err)
	}

	if stderrBuf.Len() > 0 {
		logrus.Debugf("external driver stderr: %s", stderrBuf.String())
	}

	*y = res
	logrus.Debugf("Pre-configured driver action completed successfully for %q", extDriverPath)
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
			extDriver.Logger.Errorf("Failed to inspect status for instance %q: %v", inst.Name, err)
			return "", err
		}
		extDriver.Logger.Debugf("Instance %q inspected successfully with status: %s", inst.Name, inst.Status)
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
	logrus.Debugf("Inspecting instance status action completed successfully for %q", extDriverPath)
	return inst.Status, nil
}
