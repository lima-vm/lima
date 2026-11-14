//go:build darwin && !no_vz

// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package vz

import (
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/lima-vm/lima/v2/pkg/driver"
	"github.com/lima-vm/lima/v2/pkg/limatype"
)

// USB 2.0 spec, table 9-8.
const usbDeviceDescriptorLength = 18

func parseUSBDeviceDescriptor(b []byte) (vendorID, productID uint16, err error) {
	if len(b) < usbDeviceDescriptorLength {
		return 0, 0, fmt.Errorf("USB device descriptor is %d bytes, want %d", len(b), usbDeviceDescriptorLength)
	}
	if b[1] != 1 {
		return 0, 0, fmt.Errorf("unexpected USB descriptor type %d, want 1 (DEVICE)", b[1])
	}
	return binary.LittleEndian.Uint16(b[8:10]), binary.LittleEndian.Uint16(b[10:12]), nil
}

// usbDeviceMatches reports whether dev matches every field set in m.
func usbDeviceMatches(m limatype.USBDeviceMatch, dev driver.USBDevice) bool {
	if m.VendorID != nil && *m.VendorID != dev.VendorID {
		return false
	}
	if m.ProductID != nil && *m.ProductID != dev.ProductID {
		return false
	}
	if m.Serial != "" && m.Serial != dev.Serial {
		return false
	}
	return true
}

func usbDeviceMatchesAny(rules []limatype.USBDeviceMatch, dev driver.USBDevice) bool {
	for _, m := range rules {
		if usbDeviceMatches(m, dev) {
			return true
		}
	}
	return false
}

// usbEnabled reports whether USB passthrough is enabled, which it is by default
// when devices are specified.
func usbEnabled(opts limatype.VZUSB) bool {
	if opts.Enabled != nil {
		return *opts.Enabled
	}
	return len(opts.Devices) > 0
}

func validateUSB(opts limatype.VZUSB) error {
	var errs []error
	if !usbEnabled(opts) && len(opts.Devices) > 0 {
		errs = append(errs, errors.New("field `vmOpts.vz.usb.devices` requires `vmOpts.vz.usb.enabled` to be true"))
	}
	for i, m := range opts.Devices {
		if m.VendorID == nil && m.ProductID == nil && m.Serial == "" {
			errs = append(errs, fmt.Errorf("field `vmOpts.vz.usb.devices[%d]` must set at least one of `vendorID`, `productID`, or `serial`", i))
		}
	}
	return errors.Join(errs...)
}
