// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

// Package usbip implements a USB/IP server over IOKit.
// The wire format is Documentation/usb/usbip_protocol.rst, big-endian.
package usbip

import (
	"bytes"
	"encoding/binary"
	"io"
)

const protocolVersion = 0x0111

const (
	opReqDevlist = 0x8005
	opRepDevlist = 0x0005
	opReqImport  = 0x8003
	opRepImport  = 0x0003
)

const (
	cmdSubmit = 0x00000001
	retSubmit = 0x00000003
	cmdUnlink = 0x00000002
	retUnlink = 0x00000004
)

const (
	dirOut = 0
	dirIn  = 1
)

const urbZeroPacket = 0x0040

// Values of enum usb_device_speed.
const (
	usbipSpeedLow   = 1
	usbipSpeedFull  = 2
	usbipSpeedHigh  = 3
	usbipSpeedSuper = 5
)

type opHeader struct {
	Version uint16
	Code    uint16
	Status  uint32
}

type usbDeviceDesc struct {
	Path                [256]byte
	Busid               [32]byte
	Busnum              uint32
	Devnum              uint32
	Speed               uint32
	IDVendor            uint16
	IDProduct           uint16
	BcdDevice           uint16
	BDeviceClass        uint8
	BDeviceSubClass     uint8
	BDeviceProtocol     uint8
	BConfigurationValue uint8
	BNumConfigurations  uint8
	BNumInterfaces      uint8
}

type urbHeader struct {
	Command              uint32
	Seqnum               uint32
	Devid                uint32
	Direction            uint32
	Ep                   uint32
	TransferFlags        uint32
	TransferBufferLength int32
	StartFrame           int32
	NumberOfPackets      int32
	Interval             int32
	Setup                [8]byte
}

func cString(b []byte) string {
	s, _, _ := bytes.Cut(b, []byte{0})
	return string(s)
}

func readOpHeader(r io.Reader) (opHeader, error) {
	var h opHeader
	err := binary.Read(r, binary.BigEndian, &h)
	return h, err
}

func writeOpHeader(w io.Writer, code uint16, status uint32) error {
	return binary.Write(w, binary.BigEndian, opHeader{
		Version: protocolVersion,
		Code:    code,
		Status:  status,
	})
}

func readURBHeader(r io.Reader) (urbHeader, error) {
	var h urbHeader
	err := binary.Read(r, binary.BigEndian, &h)
	return h, err
}

func writeRetSubmit(w io.Writer, h urbHeader, status, actualLength int32, data []byte) error {
	err := binary.Write(w, binary.BigEndian, urbHeader{
		Command:              retSubmit,
		Seqnum:               h.Seqnum,
		TransferFlags:        uint32(status),
		TransferBufferLength: actualLength,
		NumberOfPackets:      h.NumberOfPackets,
	})
	if err == nil && h.Direction == dirIn && actualLength > 0 {
		_, err = w.Write(data[:actualLength])
	}
	return err
}

func writeRetUnlink(w io.Writer, h urbHeader, status int32) error {
	return binary.Write(w, binary.BigEndian, urbHeader{Command: retUnlink, Seqnum: h.Seqnum, TransferFlags: uint32(status)})
}
