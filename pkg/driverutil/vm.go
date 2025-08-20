package driverutil

import (
	"encoding/json"
	"fmt"
	"os/exec"

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
	extDriver, intDriver, exists := registry.Get(vmType)
	if !exists {
		return fmt.Errorf("vmType %q is not a registered driver", vmType)
	}

	if extDriver != nil {
		if err := handlePreConfiguredDriverAction(y, extDriver.Path, filePath); err != nil {
			return fmt.Errorf("error handling pre-configured driver action for %q: %w", extDriver.Name, err)
		}

		return nil
	}

	if err := intDriver.AcceptConfig(y, filePath); err != nil {
		return err
	}
	if _, err := intDriver.FillConfig(y, filePath); err != nil {
		return err
	}

	return nil
}

func handlePreConfiguredDriverAction(y *limatype.LimaYAML, extDriverPath, filePath string) error {
	cmd := exec.Command(extDriverPath, "--pre-driver-action")
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

	logrus.Infof("Arch Type: %s, Driver: %s", *y.Arch, extDriverPath)
	encoder := json.NewEncoder(stdin)
	if err := encoder.Encode(limatype.PreConfiguredDriverPayload{
		Config:   *y,
		FilePath: filePath,
	}); err != nil {
		return fmt.Errorf("error encoding payload for pre-configured driver action: %w", err)
	}
	stdin.Close()

	decoder := json.NewDecoder(stdout)
	var response limatype.PreConfiguredDriverResponse
	if err := decoder.Decode(&response); err != nil {
		return fmt.Errorf("error decoding response from pre-configured driver action: %w", err)
	}

	if err := cmd.Wait(); err != nil {
		return err
	}

	if response.Error != "" {
		return fmt.Errorf("error from pre-configured driver action: %s", response.Error)
	}

	logrus.Infof("Received response from pre-configured driver action: %s + %s", *response.Config.Arch, response.Error)
	*y = response.Config
	logrus.Debugf("Pre-configured driver action completed successfully for %q", extDriverPath)
	return nil
}
