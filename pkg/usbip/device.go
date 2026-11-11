// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package usbip

import (
	"context"
	"errors"
)

var ErrDeviceGone = errors.New("usb device disconnected")

type DeviceInfo struct {
	BusNum      uint32
	DevNum      uint32
	BusID       string
	RegistryID  uint64
	Speed       uint32
	Vendor      uint16
	Product     uint16
	VendorName  string
	ProductName string
}

type Device interface {
	Info() DeviceInfo
	Control(ctx context.Context, setup [8]byte, data []byte) (int, error)
	Transfer(ctx context.Context, ep uint8, in bool, buf []byte) (int, error)
	// OutMaxPacket is 0 if unknown.
	OutMaxPacket(ep uint8) int
	// Gone is true even if the device came straight back.
	Gone() bool
	Close() error
}
