// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package limayaml

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	logrustest "github.com/sirupsen/logrus/hooks/test"
	"gotest.tools/v3/assert"

	"github.com/lima-vm/lima/v2/pkg/limatype"
)

func usbYAML(usb string) string {
	return "images: [{location: /}]\nusb: " + usb + "\n"
}

func TestFillDefaultUSB(t *testing.T) {
	a := []limatype.USBDevice{{VendorID: "0403", ProductID: "6015"}}
	b := []limatype.USBDevice{{VendorID: "0bda", ProductID: "8812"}}
	on, off := new(true), new(false)
	for name, tc := range map[string]struct {
		y, d, o, want limatype.USB
	}{
		"nothing":           {},
		"empty devices":     {y: limatype.USB{Devices: []limatype.USBDevice{}}},
		"devices":           {y: limatype.USB{Devices: a}, want: limatype.USB{Enabled: on, Devices: a}},
		"off, devices":      {y: limatype.USB{Enabled: off, Devices: a}, want: limatype.USB{Enabled: off, Devices: a}},
		"default":           {d: limatype.USB{Enabled: on}, want: limatype.USB{Enabled: on}},
		"instance, default": {y: limatype.USB{Enabled: off}, d: limatype.USB{Enabled: on}, want: limatype.USB{Enabled: off}},
		"override":          {y: limatype.USB{Enabled: on}, o: limatype.USB{Enabled: off}, want: limatype.USB{Enabled: off}},
		"devices combine": {
			y: limatype.USB{Devices: a}, d: limatype.USB{Devices: b}, o: limatype.USB{Devices: b},
			want: limatype.USB{Enabled: on, Devices: slices.Concat(b, a, b)},
		},
	} {
		t.Run(name, func(t *testing.T) {
			y, d, o := limatype.LimaYAML{USB: tc.y}, limatype.LimaYAML{USB: tc.d}, limatype.LimaYAML{USB: tc.o}
			FillDefault(t.Context(), &y, &d, &o, filepath.Join(t.TempDir(), "lima.yaml"), false)
			assert.DeepEqual(t, y.USB, tc.want)
		})
	}
}

func TestValidateUSBWarnings(t *testing.T) {
	for name, tc := range map[string]struct {
		yaml string
		want string
	}{
		"experimental": {usbYAML("{enabled: true}"), "`usb` is experimental"},
		"plain":        {"plain: true\n" + usbYAML("{enabled: true}"), "ignored with `plain: true`"},
		"macOS":        {"os: Darwin\nvmType: vz\n" + usbYAML("{enabled: true}"), "needs a Linux guest"},
		"FreeBSD":      {"os: FreeBSD\nvmType: vz\n" + usbYAML("{enabled: true}"), "needs a Linux guest"},
		"qemu":         {"vmType: qemu\n" + usbYAML("{enabled: true}"), "only supported for vmType vz"},
	} {
		t.Run(name, func(t *testing.T) {
			y, err := Load(t.Context(), []byte(tc.yaml), "usb.yaml")
			assert.NilError(t, err)
			hook := logrustest.NewGlobal()
			assert.NilError(t, ValidateUSB(y, false))
			assert.Equal(t, len(hook.AllEntries()), 0)
			assert.NilError(t, ValidateUSB(y, true))
			found := false
			for _, e := range hook.AllEntries() {
				found = found || strings.Contains(e.Message, tc.want)
			}
			assert.Assert(t, found)
		})
	}
}

func TestValidateUSB(t *testing.T) {
	device := func(fields string) string { return usbYAML("{devices: [{" + fields + "}]}") }
	for name, tc := range map[string]struct {
		yaml, wantErr string
	}{
		"no usb":          {`images: [{"location": "/"}]`, ""},
		"ids":             {device(`vendorID: "0bda", productID: "8812"`), ""},
		"upper case":      {device(`vendorID: "0BDA", productID: "8812"`), ""},
		"number-like":     {device(`vendorID: "0403", productID: "1e71"`), ""},
		"busid":           {device(`vendorID: "0403", productID: "6015", busAddr: "1-0x01100000"`), ""},
		"short vendor":    {device(`vendorID: "403", productID: "6015"`), "usb.devices[0].vendorID` must be 4 hex digits"},
		"short product":   {device(`vendorID: "0403", productID: "15"`), "usb.devices[0].productID` must be 4 hex digits"},
		"not hex":         {device(`vendorID: "zzzz", productID: "6015"`), "vendorID` must be 4 hex digits"},
		"0x literal":      {device(`vendorID: 0x0403, productID: "6015"`), "vendorID` must be 4 hex digits in quotes, got 1027"},
		"0x product":      {device(`vendorID: "0403", productID: 0x0a0b`), "productID` must be 4 hex digits in quotes, got 2571"},
		"missing":         {device(`vendorID: "0403"`), "productID` must be 4 hex digits"},
		"short busAddr":   {device(`vendorID: "0403", productID: "6015", busAddr: "1-0x0110"`), "busAddr` must be a BUSID"},
		"upper busAddr":   {device(`vendorID: "0403", productID: "6015", busAddr: "1-0x0110000A"`), "busAddr` must be a BUSID"},
		"off, bad device": {usbYAML("{enabled: false, devices: [{vendorID: x, productID: y}]}"), "vendorID` must be 4 hex digits"},
	} {
		t.Run(name, func(t *testing.T) {
			y, err := Load(t.Context(), []byte(tc.yaml), "usb.yaml")
			assert.NilError(t, err)
			err = ValidateUSB(y, false)
			if tc.wantErr == "" {
				assert.NilError(t, err)
			} else {
				assert.ErrorContains(t, err, tc.wantErr)
			}
		})
	}
}

func TestValidateChecksUSBForEveryVMType(t *testing.T) {
	for _, vmType := range []limatype.VMType{limatype.VZ, limatype.QEMU} {
		t.Run(vmType, func(t *testing.T) {
			y, err := Load(t.Context(), []byte("vmType: "+vmType+"\n"+usbYAML(`{devices: [{vendorID: "403", productID: "6015"}]}`)), "usb.yaml")
			assert.NilError(t, err)
			assert.ErrorContains(t, Validate(y, false), "usb.devices[0].vendorID` must be 4 hex digits")
		})
	}
}
