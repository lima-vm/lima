// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package apfs

import (
	"encoding/binary"
	"fmt"
)

const fletcher64Mod = 0xFFFFFFFF

// fletcher64 computes the APFS Fletcher-64 checksum over block[8:].
func fletcher64(block []byte) uint64 {
	var sum1, sum2 uint64
	for i := 8; i+3 < len(block); i += 4 {
		sum1 = (sum1 + uint64(binary.LittleEndian.Uint32(block[i:]))) % fletcher64Mod
		sum2 = (sum2 + sum1) % fletcher64Mod
	}
	ckLow := fletcher64Mod - ((sum1 + sum2) % fletcher64Mod)
	ckHigh := fletcher64Mod - ((sum1 + ckLow) % fletcher64Mod)
	return (ckHigh << 32) | ckLow
}

// verifyChecksum returns an error if the stored checksum does not match.
func verifyChecksum(block []byte) error {
	stored := binary.LittleEndian.Uint64(block[objChecksumOff:])
	computed := fletcher64(block)
	if stored != computed {
		return fmt.Errorf("checksum mismatch: stored %#x, computed %#x", stored, computed)
	}
	return nil
}

// updateChecksum recomputes and stores the Fletcher-64 checksum.
func updateChecksum(block []byte) {
	binary.LittleEndian.PutUint64(block[objChecksumOff:], fletcher64(block))
}
