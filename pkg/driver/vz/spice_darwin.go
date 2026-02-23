// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

//go:build darwin

package vz

import (
	"github.com/Code-Hex/vz/v3"
	"github.com/sirupsen/logrus"

	"github.com/lima-vm/lima/v2/pkg/limatype"
)

// attachSpiceAgent configures SPICE agent for clipboard sharing.
// This enables bidirectional clipboard sharing between host and guest.
// SPICE agent requires a display to be useful.
func attachSpiceAgent(inst *limatype.Instance, vmConfig *vz.VirtualMachineConfiguration) error {
	// SPICE agent only makes sense with a display
	if inst.Config.Video.Display == nil || *inst.Config.Video.Display == "none" {
		return nil
	}

	// Check clipboard configuration
	// Default: enabled when display is present
	// Respect explicit user configuration (true or false)
	if inst.Config.Video.Clipboard != nil && !*inst.Config.Video.Clipboard {
		logrus.Debug("Clipboard sharing explicitly disabled in configuration")
		return nil
	}

	// Get the SPICE agent port name
	portName, err := vz.SpiceAgentPortAttachmentName()
	if err != nil {
		logrus.Warnf("Failed to get SPICE agent port name: %v", err)
		return nil // Not fatal, clipboard just won't work
	}

	// Create SPICE agent port attachment
	spiceAgent, err := vz.NewSpiceAgentPortAttachment()
	if err != nil {
		logrus.Warnf("Failed to create SPICE agent: %v", err)
		return nil // Not fatal, clipboard just won't work
	}

	// Enable clipboard sharing
	spiceAgent.SetSharesClipboard(true)

	// Create virtio console device if not already configured
	consoleDevice, err := vz.NewVirtioConsoleDeviceConfiguration()
	if err != nil {
		logrus.Warnf("Failed to create console device for SPICE: %v", err)
		return nil
	}

	// Create console port configuration with SPICE agent
	portConfig, err := vz.NewVirtioConsolePortConfiguration(
		vz.WithVirtioConsolePortConfigurationName(portName),
		vz.WithVirtioConsolePortConfigurationAttachment(spiceAgent),
	)
	if err != nil {
		logrus.Warnf("Failed to create SPICE agent port configuration: %v", err)
		return nil
	}

	// Attach the port to the console device
	// Port 0 is typically the first console port
	consoleDevice.SetVirtioConsolePortConfiguration(0, portConfig)

	// Set the console device in the VM configuration
	vmConfig.SetConsoleDevicesVirtualMachineConfiguration([]vz.ConsoleDeviceConfiguration{
		consoleDevice,
	})

	logrus.Info("SPICE agent configured for clipboard sharing")
	return nil
}
