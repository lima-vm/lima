// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package limatype

// USB configures USB/IP passthrough.
type USB struct {
	Enabled *bool       `yaml:"enabled,omitempty" json:"enabled,omitempty" jsonschema:"nullable"`
	Devices []USBDevice `yaml:"devices,omitempty" json:"devices,omitempty"`
}

// USBDevice selects a host USB device imported at boot.
type USBDevice struct {
	// Name is an optional label for the device.
	Name string `yaml:"name,omitempty" json:"name,omitempty"`
	// VendorID is a string of exactly 4 hex digits, e.g. "0bda". It is any because YAML
	// decodes unquoted 0x0403 into a string as "1027", which validation could not refuse.
	VendorID any `yaml:"vendorID" json:"vendorID"`
	// ProductID is like VendorID, e.g. "8812".
	ProductID any `yaml:"productID" json:"productID"`
	// BusAddr optionally picks one of several identical devices. It is a host BUSID:
	// the bus number and IOKit locationID, e.g. "1-0x01100000".
	BusAddr string `yaml:"busAddr,omitempty" json:"busAddr,omitempty"`
}
