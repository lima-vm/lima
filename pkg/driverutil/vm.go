package driverutil

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"

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
		if err := handlePreConfiguredDriverAction(y, extDriver.Path, filePath); err != nil {
			return err
		}

		return nil
	}

	if err := intDriver.AcceptConfig(y, filePath); err != nil {
		return err
	}
	if err := intDriver.FillConfig(y, filePath); err != nil {
		return err
	}

	return nil
}

func handlePreConfiguredDriverAction(y *limatype.LimaYAML, extDriverPath, filePath string) error {
	cmd := exec.CommandContext(context.Background(), extDriverPath, "--pre-driver-action")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}

	encoder := json.NewEncoder(stdin)
	if err := encoder.Encode(limatype.PreConfiguredDriverPayload{
		Config:   *y,
		FilePath: filePath,
	}); err != nil {
		return err
	}
	stdin.Close()

	decoder := json.NewDecoder(stdout)
	var response limatype.PreConfiguredDriverResponse
	if err := decoder.Decode(&response); err != nil {
		return err
	}

	if err := cmd.Wait(); err != nil {
		return err
	}

	if response.Error != "" {
		return errors.New(response.Error)
	}

	*y = response.Config
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
		status, err := handleInspectStatusAction(inst, extDriver.Path)
		if err != nil {
			extDriver.Logger.Errorf("Failed to inspect status for instance %q: %v", inst.Name, err)
			return "", err
		}
		extDriver.Logger.Debugf("Instance %q inspected successfully with status: %s", inst.Name, inst.Status)
		return status, nil
	}

	return intDriver.InspectStatus(ctx, inst), nil
}

func handleInspectStatusAction(inst *limatype.Instance, extDriverPath string) (string, error) {
	cmd := exec.CommandContext(context.Background(), extDriverPath, "--inspect-status")
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
		return "", err
	}

	*inst = respInst
	logrus.Debugf("Inspecting instance status action completed successfully for %q", extDriverPath)
	return inst.Status, nil
}
