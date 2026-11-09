// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package usbip

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"testing"

	"gotest.tools/v3/assert"
)

const urbHeaderSize = 48

func TestReadURBHeaderRoundTrip(t *testing.T) {
	// CMD_SUBMIT per usbip_protocol.rst; every field differs so a swapped offset shows.
	frame, err := hex.DecodeString("00000001" + "0000002a" + "00030007" + "00000001" + "00000003" + "00000200" +
		"00000040" + "00000006" + "00000005" + "0000000a" + "8006000100001200")
	assert.NilError(t, err)
	got, err := readURBHeader(bytes.NewReader(frame))
	assert.NilError(t, err)
	assert.DeepEqual(t, got, urbHeader{
		Command: cmdSubmit, Seqnum: 42, Devid: 3<<16 | 7, Direction: dirIn, Ep: 3, TransferFlags: 0x200,
		TransferBufferLength: 64, StartFrame: 6, NumberOfPackets: 5, Interval: 10,
		Setup: [8]byte{0x80, 0x06, 0x00, 0x01, 0x00, 0x00, 0x12, 0x00},
	})
}

func TestWriteRetSubmitPayload(t *testing.T) {
	// IN carries actual_length bytes, OUT none.
	for dir, want := range map[uint32][]byte{dirIn: {1, 2}, dirOut: {}} {
		var out bytes.Buffer
		assert.NilError(t, writeRetSubmit(&out, urbHeader{Direction: dir}, 0, 2, []byte{1, 2, 3, 4}))
		assert.DeepEqual(t, out.Bytes()[urbHeaderSize:], want)
	}
}

func TestDevlistWrite(t *testing.T) {
	info := DeviceInfo{
		Busid:   "20-3",
		BusNum:  20,
		DevNum:  3,
		Speed:   usbipSpeedHigh,
		Vendor:  0x0bda,
		Product: 0x8812,
	}
	var out bytes.Buffer
	assert.NilError(t, writeDevlist(&out, []DeviceInfo{info}))

	b := out.Bytes()
	assert.Equal(t, len(b), 8+4+312)
	assert.Equal(t, binary.BigEndian.Uint16(b[2:]), uint16(opRepDevlist))
	assert.Equal(t, binary.BigEndian.Uint32(b[8:]), uint32(1))
	// Offsets from OP_REP_DEVLIST in usbip_protocol.rst.
	assert.Equal(t, cString(b[0x10c:0x12c]), info.Busid)
	assert.Equal(t, binary.BigEndian.Uint32(b[0x12c:]), info.BusNum)
	assert.Equal(t, binary.BigEndian.Uint32(b[0x130:]), info.DevNum)
	assert.Equal(t, binary.BigEndian.Uint32(b[0x134:]), info.Speed)
}
