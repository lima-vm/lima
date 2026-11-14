// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package hostagent

import (
	"context"
	"fmt"

	"github.com/lima-vm/lima/v2/pkg/driver"
)

func (a *HostAgent) usbPassthrough(ctx context.Context) (driver.USBPassthrough, error) {
	// ConfiguredDriver wraps the concrete driver; unwrap to reach optional interfaces.
	inner := a.driver
	if cd, ok := a.driver.(*driver.ConfiguredDriver); ok {
		inner = cd.Driver
	}
	p, ok := inner.(driver.USBPassthrough)
	if !ok {
		return nil, fmt.Errorf("driver %q: %w", a.driver.Info(ctx).Name, driver.ErrDriverNotUSBPassthrough)
	}
	return p, nil
}

func (a *HostAgent) ListUSBDevices(ctx context.Context) ([]driver.USBDevice, error) {
	p, err := a.usbPassthrough(ctx)
	if err != nil {
		return nil, err
	}
	return p.ListUSBDevices(ctx)
}

func (a *HostAgent) AttachUSBDevice(ctx context.Context, registryID uint64) error {
	p, err := a.usbPassthrough(ctx)
	if err != nil {
		return err
	}
	return p.AttachUSBDevice(ctx, registryID)
}

func (a *HostAgent) DetachUSBDevice(ctx context.Context, registryID uint64) error {
	p, err := a.usbPassthrough(ctx)
	if err != nil {
		return err
	}
	return p.DetachUSBDevice(ctx, registryID)
}
