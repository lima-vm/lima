// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package apfs

import (
	"encoding/binary"
	"testing"

	"gotest.tools/v3/assert"
)

func TestFletcher64(t *testing.T) {
	// Construct a minimal 4096-byte block and verify the checksum
	// round-trips: compute -> store -> verify.
	block := make([]byte, 4096)
	for i := 8; i < len(block); i++ {
		block[i] = byte(i % 251)
	}
	updateChecksum(block)
	assert.NilError(t, verifyChecksum(block))

	// Corrupt one byte and verify detection.
	block[100]++
	assert.Assert(t, verifyChecksum(block) != nil, "verifyChecksum should have failed after corruption")
}

func TestFletcher64KnownVector(t *testing.T) {
	// A block of all zeros (except checksum) should produce a
	// deterministic checksum. Verify consistency.
	block := make([]byte, 4096)
	ck := fletcher64(block)
	updateChecksum(block)
	stored := binary.LittleEndian.Uint64(block[0:])
	assert.Equal(t, stored, ck)
	assert.NilError(t, verifyChecksum(block))
}
