// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"errors"
	"io"
	"testing"

	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gotest.tools/v3/assert"

	"github.com/lima-vm/lima/v2/pkg/driver"
)

func TestUSBError(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(io.Discard)
	d := &DriverClient{logger: logger}
	usbError := func(err error) error { return d.usbError(err, "test") }

	err := usbError(status.Error(codes.Unimplemented, driver.ErrDriverNotUSBPassthrough.Error()))
	assert.Assert(t, errors.Is(err, driver.ErrDriverNotUSBPassthrough))
	assert.Equal(t, err.Error(), driver.ErrDriverNotUSBPassthrough.Error())

	err = usbError(status.Error(codes.Unimplemented, driver.ErrDriverNotUSBPassthrough.Error()+": requires macOS 27"))
	assert.Assert(t, errors.Is(err, driver.ErrDriverNotUSBPassthrough))
	assert.Equal(t, err.Error(), "driver does not support USB passthrough: requires macOS 27")

	err = usbError(status.Error(codes.Unknown, "USB device 0x1 is already attached"))
	assert.Assert(t, !errors.Is(err, driver.ErrDriverNotUSBPassthrough))
	assert.Equal(t, err.Error(), "USB device 0x1 is already attached")
}
