//go:build darwin && !no_vz

// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package vz

import (
	"context"

	"github.com/Code-Hex/vz/v3"
	"github.com/lima-vm/lima/v2/pkg/limatype"
)

func getMacMachineIdentifier(_ string) (machineIdentifier, error) {
	return nil, errMacOSGuestUnsupported
}

func newMacPlatformConfiguration(_ *limatype.Instance) (vz.PlatformConfiguration, error) {
	return nil, errMacOSGuestUnsupported
}

func newMacGraphicsDeviceConfiguration(_, _, _ int64) (vz.GraphicsDeviceConfiguration, error) {
	return nil, errMacOSGuestUnsupported
}

func newMacPointingDeviceConfiguration() (vz.PointingDeviceConfiguration, error) {
	return nil, errMacOSGuestUnsupported
}

func newMacKeyboardConfiguration() (vz.KeyboardConfiguration, error) {
	return nil, errMacOSGuestUnsupported
}

func newMacOSBootLoader() (vz.BootLoader, error) {
	return nil, errMacOSGuestUnsupported
}

func installMacOS(_ context.Context, _ *vz.VirtualMachine, _ string) error {
	return errMacOSGuestUnsupported
}
