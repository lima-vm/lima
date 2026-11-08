// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package limatype

// USBDevice selects a host USB device imported at boot.
type USBDevice struct {
	// Name is an optional label shown by `limactl usb list`.
	Name string `yaml:"name,omitempty" json:"name,omitempty"`
	// VendorID is a string of exactly 4 hex digits, e.g. "0bda". It is any because vmOpts
	// is already decoded: unquoted 0x0403 is the number 1027 here, which validation refuses.
	VendorID any `yaml:"vendorID" json:"vendorID"`
	// ProductID is like VendorID, e.g. "8812".
	ProductID any `yaml:"productID" json:"productID"`
	// BusAddr optionally picks one of several identical devices. It is a
	// BUSID from `limactl usb list`, e.g. "20-0x14300000".
	BusAddr string `yaml:"busAddr,omitempty" json:"busAddr,omitempty"`
}
