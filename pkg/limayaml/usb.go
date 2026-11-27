// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package limayaml

import (
	"errors"
	"fmt"
	"regexp"

	"github.com/sirupsen/logrus"

	"github.com/lima-vm/lima/v2/pkg/limatype"
)

var (
	usbIDRegexp    = regexp.MustCompile(`^[0-9a-fA-F]{4}$`)
	usbBusidRegexp = regexp.MustCompile(`^\d+-0x[0-9a-f]{8}$`)
)

func validUSBID(id any) bool {
	s, ok := id.(string)
	return ok && usbIDRegexp.MatchString(s)
}

func ValidateUSB(y *limatype.LimaYAML, warn bool) error {
	var errs error
	for i, dev := range y.USB.Devices {
		if !validUSBID(dev.VendorID) {
			errs = errors.Join(errs, fmt.Errorf("field `usb.devices[%d].vendorID` must be 4 hex digits in quotes, got %v", i, dev.VendorID))
		}
		if !validUSBID(dev.ProductID) {
			errs = errors.Join(errs, fmt.Errorf("field `usb.devices[%d].productID` must be 4 hex digits in quotes, got %v", i, dev.ProductID))
		}
		if dev.BusAddr != "" && !usbBusidRegexp.MatchString(dev.BusAddr) {
			errs = errors.Join(errs, fmt.Errorf("field `usb.devices[%d].busAddr` must be a BUSID such as `1-0x01100000`, got %#q", i, dev.BusAddr))
		}
	}
	if !warn || y.USB.Enabled == nil || !*y.USB.Enabled {
		return errs
	}
	if y.VMType != nil && *y.VMType != limatype.VZ {
		logrus.Warnf("`usb` is currently only supported for vmType vz; ignored for vmType %#q", *y.VMType)
		return errs
	}
	logrus.Warn("`usb` is experimental")
	if *y.Plain {
		logrus.Warn("`usb` needs the guest agent; ignored with `plain: true`")
	}
	if *y.OS != limatype.LINUX {
		logrus.Warnf("`usb` needs a Linux guest; ignored for os %#q", *y.OS)
	}
	return errs
}
