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

// USB reads vmOpts.vz.usb. `devices` alone implies enabled.
func USB(y *limatype.LimaYAML) limatype.USB {
	var vzOpts limatype.VZOpts
	if err := Convert(y.VMOpts[limatype.VZ], &vzOpts, "vmOpts.vz"); err != nil {
		logrus.WithError(err).Warn("Couldn't convert vmOpts.vz")
	}
	usb := vzOpts.USB
	if usb.Enabled == nil && len(usb.Devices) > 0 {
		usb.Enabled = new(true)
	}
	return usb
}

func validUSBID(id any) bool {
	s, ok := id.(string)
	return ok && usbIDRegexp.MatchString(s)
}

func ValidateUSB(y *limatype.LimaYAML) error {
	usb := USB(y)
	var errs error
	for i, dev := range usb.Devices {
		if !validUSBID(dev.VendorID) {
			errs = errors.Join(errs, fmt.Errorf("field `vmOpts.vz.usb.devices[%d].vendorID` must be 4 hex digits in quotes, got %v", i, dev.VendorID))
		}
		if !validUSBID(dev.ProductID) {
			errs = errors.Join(errs, fmt.Errorf("field `vmOpts.vz.usb.devices[%d].productID` must be 4 hex digits in quotes, got %v", i, dev.ProductID))
		}
		if dev.BusAddr != "" && !usbBusidRegexp.MatchString(dev.BusAddr) {
			errs = errors.Join(errs, fmt.Errorf("field `vmOpts.vz.usb.devices[%d].busAddr` must be a BUSID from `limactl usb list`, e.g. 20-0x14300000, got %#q", i, dev.BusAddr))
		}
	}
	if usb.Enabled == nil || !*usb.Enabled {
		return errs
	}
	logrus.Warn("`vmOpts.vz.usb` is experimental")
	if *y.Plain {
		logrus.Warn("`vmOpts.vz.usb` needs the guest agent; ignored with `plain: true`")
	}
	if *y.OS != limatype.LINUX {
		logrus.Warnf("`vmOpts.vz.usb` needs a Linux guest; ignored for os %#q", *y.OS)
	}
	return errs
}
