// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

// Package usbip implements the USB/IP protocol.
// The wire format, big-endian, is the Linux kernel's https://docs.kernel.org/usb/usbip_protocol.html.
package usbip

import (
	"encoding/binary"
	"io"
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
