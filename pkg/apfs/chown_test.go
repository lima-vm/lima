// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package apfs

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"gotest.tools/v3/assert"
)

// writeGPTImage builds a disk file with a GPT header at LBA1 carrying the
// given partition-array fields, plus optional entry bytes at partEntryLBA.
func writeGPTImage(t *testing.T, partEntryLBA uint64, numEntries, entrySize uint32, entries []byte) *os.File {
	t.Helper()
	size := max(int64(partEntryLBA)*gptLBASectorSize+int64(len(entries)), 4096)
	buf := make([]byte, size)
	copy(buf[gptLBASectorSize:], gptHeaderSignature)
	le.PutUint64(buf[gptLBASectorSize+72:], partEntryLBA)
	le.PutUint32(buf[gptLBASectorSize+80:], numEntries)
	le.PutUint32(buf[gptLBASectorSize+84:], entrySize)
	copy(buf[int64(partEntryLBA)*gptLBASectorSize:], entries)

	path := filepath.Join(t.TempDir(), "disk.img")
	assert.NilError(t, os.WriteFile(path, buf, 0o600))
	f, err := os.Open(path)
	assert.NilError(t, err)
	t.Cleanup(func() { _ = f.Close() })
	return f
}

func TestFindAPFSPartitionGPTRejectsBadHeader(t *testing.T) {
	// entrySize and numEntries are read straight from the GPT header. An
	// entrySize below 16 makes the entryBuf[0:16] type-GUID read run past the
	// buffer, and an oversized value would size a multi-GB allocation, so a
	// crafted disk must be rejected rather than panic.
	for _, tc := range []struct {
		name       string
		numEntries uint32
		entrySize  uint32
	}{
		{"entry-size-too-small", 4, 8},
		{"entry-size-zero", 4, 0},
		{"entry-size-too-large", 4, gptMaxEntrySize + 1},
		{"table-too-large", gptMaxTableSize/gptMinEntrySize + 1, gptMinEntrySize},
		{"table-too-large-max-entry-size", gptMaxTableSize/gptMaxEntrySize + 1, gptMaxEntrySize},
		{"entry-count-zero", 0, gptMinEntrySize},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := writeGPTImage(t, 2, tc.numEntries, tc.entrySize, nil)
			_, err := findAPFSPartitionGPT(f)
			// Match the guard's own error: with no entry bytes in the image
			// an unguarded parser also fails, with EOF or "no APFS partition".
			assert.ErrorContains(t, err, "out of range")
		})
	}
}

func TestFindAPFSPartitionGPTFindsAPFS(t *testing.T) {
	// UEFI sets no maximum on the entry count (128 is only the default), so a
	// larger table must still be scanned through to its last entry. The last
	// case is a table of exactly gptMaxTableSize built from gptMaxEntrySize
	// entries, the accepted edge of both bounds.
	for _, tc := range []struct {
		numEntries uint32
		entrySize  uint32
	}{
		{1, gptMinEntrySize},
		{256, gptMinEntrySize},
		{gptMaxTableSize / gptMaxEntrySize, gptMaxEntrySize},
	} {
		name := strconv.Itoa(int(tc.numEntries)) + "x" + strconv.Itoa(int(tc.entrySize))
		t.Run(name, func(t *testing.T) {
			n, size := int(tc.numEntries), int(tc.entrySize)
			entries := make([]byte, n*size)
			// Fill the leading entries with a non-empty, non-APFS type so the
			// scan does not stop early at an unused entry.
			for i := range n - 1 {
				entries[i*size] = 1
			}
			last := entries[(n-1)*size:]
			copy(last[0:16], apfsPartTypeGUID[:])
			le.PutUint64(last[32:], 40) // firstLBA

			f := writeGPTImage(t, 2, tc.numEntries, tc.entrySize, entries)
			off, err := findAPFSPartitionGPT(f)
			assert.NilError(t, err)
			assert.Equal(t, off, int64(40)*gptLBASectorSize)
		})
	}
}

func TestOpenContainerBlockSize(t *testing.T) {
	// nx_block_size sizes every readBlock allocation, so openContainer must
	// reject values outside [4096, nxMaxBlockSize] before any block is read.
	for _, tc := range []struct {
		blockSize uint32
		ok        bool
	}{
		{4096, true},
		{nxMaxBlockSize, true},
		{4095, false},
		{nxMaxBlockSize + 1, false},
		{0xFFFFFFFF, false},
	} {
		t.Run(strconv.FormatUint(uint64(tc.blockSize), 10), func(t *testing.T) {
			hdr := make([]byte, 4096)
			le.PutUint32(hdr[nxMagicOff:], nxMagic)
			le.PutUint32(hdr[nxBlockSizeOff:], tc.blockSize)
			path := filepath.Join(t.TempDir(), "container.img")
			assert.NilError(t, os.WriteFile(path, hdr, 0o600))

			c, err := openContainer(path)
			if !tc.ok {
				assert.ErrorContains(t, err, "invalid block size")
				return
			}
			assert.NilError(t, err)
			defer c.close()
			assert.Equal(t, c.blockSize, tc.blockSize)
		})
	}
}
