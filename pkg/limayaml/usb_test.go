// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package limayaml

import (
	"strings"
	"testing"

	logrustest "github.com/sirupsen/logrus/hooks/test"
	"gotest.tools/v3/assert"

	"github.com/lima-vm/lima/v2/pkg/limatype"
)

func vmOpts(usb string) string {
	return "images: [{location: /}]\nvmOpts: {vz: {usb: " + usb + "}}\n"
}

func TestUSBFromVMOpts(t *testing.T) {
	a := limatype.USBDevice{VendorID: "0403", ProductID: "6015"}
	dev := `devices: [{vendorID: "0403", productID: "6015"}]`
	for name, tc := range map[string]struct {
		yaml string
		want limatype.USB
	}{
		"nothing":       {"images: [{location: /}]", limatype.USB{}},
		"empty devices": {vmOpts("{devices: []}"), limatype.USB{}},
		"devices":       {vmOpts("{" + dev + "}"), limatype.USB{Enabled: new(true), Devices: []limatype.USBDevice{a}}},
		"off, devices":  {vmOpts("{enabled: false, " + dev + "}"), limatype.USB{Enabled: new(false), Devices: []limatype.USBDevice{a}}},
	} {
		t.Run(name, func(t *testing.T) {
			y, err := Load(t.Context(), []byte(tc.yaml), "usb.yaml")
			assert.NilError(t, err)
			got := USB(y)
			if len(got.Devices) == 0 {
				got.Devices = nil
			}
			assert.DeepEqual(t, got, tc.want)
		})
	}
}

func TestUSBUnknownKeyLeavesUSBOff(t *testing.T) {
	dev := `vendorID: "0403", productID: "6015"`
	for _, usb := range []string{
		`{devices: [{` + dev + `, busAdr: "1-0x01100000"}]}`,
		`{enable: false, devices: [{` + dev + `}]}`,
	} {
		hook := logrustest.NewGlobal()
		y, err := Load(t.Context(), []byte(vmOpts(usb)), "usb.yaml")
		assert.NilError(t, err)
		assert.DeepEqual(t, USB(y), limatype.USB{})
		assert.ErrorContains(t, hook.LastEntry().Data["error"].(error), "unknown field")
	}
}

func TestValidateUSBWarnings(t *testing.T) {
	for name, tc := range map[string]struct {
		yaml string
		want string
	}{
		"experimental": {vmOpts("{enabled: true}"), "`vmOpts.vz.usb` is experimental"},
		"plain":        {"plain: true\n" + vmOpts("{enabled: true}"), "ignored with `plain: true`"},
		"macOS":        {"os: Darwin\nvmType: vz\n" + vmOpts("{enabled: true}"), "needs a Linux guest"},
		"FreeBSD":      {"os: FreeBSD\nvmType: vz\n" + vmOpts("{enabled: true}"), "needs a Linux guest"},
	} {
		t.Run(name, func(t *testing.T) {
			hook := logrustest.NewGlobal()
			y, err := Load(t.Context(), []byte(tc.yaml), "usb.yaml")
			assert.NilError(t, err)
			assert.NilError(t, ValidateUSB(y))
			found := false
			for _, e := range hook.AllEntries() {
				found = found || strings.Contains(e.Message, tc.want)
			}
			assert.Assert(t, found)
		})
	}
}

func TestValidateUSB(t *testing.T) {
	device := func(fields string) string { return vmOpts("{devices: [{" + fields + "}]}") }
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
		"off, bad device": {vmOpts("{enabled: false, devices: [{vendorID: x, productID: y}]}"), "vendorID` must be 4 hex digits"},
	} {
		t.Run(name, func(t *testing.T) {
			y, err := Load(t.Context(), []byte(tc.yaml), "usb.yaml")
			assert.NilError(t, err)
			err = ValidateUSB(y)
			if tc.wantErr == "" {
				assert.NilError(t, err)
			} else {
				assert.ErrorContains(t, err, tc.wantErr)
			}
		})
	}
}
