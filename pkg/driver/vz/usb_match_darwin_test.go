//go:build darwin && !no_vz

// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package vz

import (
	"testing"

	"gotest.tools/v3/assert"

	"github.com/lima-vm/lima/v2/pkg/driver"
	"github.com/lima-vm/lima/v2/pkg/limatype"
	"github.com/lima-vm/lima/v2/pkg/limayaml"
)

func TestParseUSBDeviceDescriptor(t *testing.T) {
	// Apple Internal Keyboard / Trackpad style descriptor: VID 0x05ac, PID 0x0342.
	desc := []byte{
		0x12, 0x01, 0x00, 0x02, 0x00, 0x00, 0x00, 0x40,
		0xac, 0x05, 0x42, 0x03, 0x00, 0x01, 0x01, 0x02,
		0x03, 0x01,
	}
	vid, pid, err := parseUSBDeviceDescriptor(desc)
	assert.NilError(t, err)
	assert.Equal(t, vid, uint16(0x05ac))
	assert.Equal(t, pid, uint16(0x0342))

	_, _, err = parseUSBDeviceDescriptor(desc[:10])
	assert.ErrorContains(t, err, "want 18")

	bad := append([]byte{}, desc...)
	bad[1] = 2
	_, _, err = parseUSBDeviceDescriptor(bad)
	assert.ErrorContains(t, err, "descriptor type 2")
}

func TestUSBDeviceMatches(t *testing.T) {
	dev := driver.USBDevice{VendorID: 0x1234, ProductID: 0x5678, Serial: "ABC"}
	tests := []struct {
		name string
		m    limatype.USBDeviceMatch
		want bool
	}{
		{"vendor", limatype.USBDeviceMatch{VendorID: new(uint16(0x1234))}, true},
		{"vendor+product", limatype.USBDeviceMatch{VendorID: new(uint16(0x1234)), ProductID: new(uint16(0x5678))}, true},
		{"all", limatype.USBDeviceMatch{VendorID: new(uint16(0x1234)), ProductID: new(uint16(0x5678)), Serial: "ABC"}, true},
		{"serial only", limatype.USBDeviceMatch{Serial: "ABC"}, true},
		{"wrong vendor", limatype.USBDeviceMatch{VendorID: new(uint16(0x1111))}, false},
		{"wrong product", limatype.USBDeviceMatch{VendorID: new(uint16(0x1234)), ProductID: new(uint16(0x1111))}, false},
		{"wrong serial", limatype.USBDeviceMatch{VendorID: new(uint16(0x1234)), Serial: "XYZ"}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, usbDeviceMatches(tc.m, dev), tc.want)
		})
	}
	assert.Assert(t, !usbDeviceMatchesAny(nil, dev))
	assert.Assert(t, usbDeviceMatchesAny([]limatype.USBDeviceMatch{{Serial: "XYZ"}, {Serial: "ABC"}}, dev))
}

func TestUSBEnabled(t *testing.T) {
	devices := []limatype.USBDeviceMatch{{Serial: "ABC"}}
	assert.Assert(t, !usbEnabled(limatype.VZUSB{}))
	assert.Assert(t, usbEnabled(limatype.VZUSB{Devices: devices}))
	assert.Assert(t, usbEnabled(limatype.VZUSB{Enabled: new(true)}))
	assert.Assert(t, !usbEnabled(limatype.VZUSB{Enabled: new(false), Devices: devices}))
}

func TestValidateUSB(t *testing.T) {
	assert.NilError(t, validateUSB(limatype.VZUSB{}))
	assert.NilError(t, validateUSB(limatype.VZUSB{Enabled: new(true)}))
	assert.NilError(t, validateUSB(limatype.VZUSB{Devices: []limatype.USBDeviceMatch{{Serial: "ABC"}}}))
	err := validateUSB(limatype.VZUSB{Devices: []limatype.USBDeviceMatch{{VendorID: new(uint16(1))}, {}}})
	assert.ErrorContains(t, err, "vmOpts.vz.usb.devices[1]")
	err = validateUSB(limatype.VZUSB{Enabled: new(false), Devices: []limatype.USBDeviceMatch{{Serial: "ABC"}}})
	assert.ErrorContains(t, err, "requires `vmOpts.vz.usb.enabled`")
}

func TestUSBOptsYAML(t *testing.T) {
	// Users copy IDs as hex from System Information, so hex literals must decode.
	y := `
vmOpts:
  vz:
    usb:
      devices:
      - vendorID: 0x05ac
        productID: 0x0342
        serial: "X1"
`
	var cfg limatype.LimaYAML
	assert.NilError(t, limayaml.Unmarshal([]byte(y), &cfg, "test"))
	var opts limatype.VZOpts
	assert.NilError(t, limayaml.Convert(cfg.VMOpts[limatype.VZ], &opts, "vmOpts.vz"))
	assert.Equal(t, len(opts.USB.Devices), 1)
	assert.Equal(t, *opts.USB.Devices[0].VendorID, uint16(0x05ac))
	assert.Equal(t, *opts.USB.Devices[0].ProductID, uint16(0x0342))
	assert.Equal(t, opts.USB.Devices[0].Serial, "X1")
}
