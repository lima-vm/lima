// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package apfs

import (
	"os"
	"path/filepath"
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
		{"entry-count-too-large", gptMaxEntries + 1, gptMinEntrySize},
		{"entry-count-zero", 0, gptMinEntrySize},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := writeGPTImage(t, 2, tc.numEntries, tc.entrySize, nil)
			_, err := findAPFSPartitionGPT(f)
			assert.Assert(t, err != nil, "expected error for out-of-range GPT header fields")
		})
	}
}

func TestFindAPFSPartitionGPTFindsAPFS(t *testing.T) {
	entry := make([]byte, gptMinEntrySize)
	copy(entry[0:16], apfsPartTypeGUID[:])
	le.PutUint64(entry[32:], 40) // firstLBA

	f := writeGPTImage(t, 2, 1, gptMinEntrySize, entry)
	off, err := findAPFSPartitionGPT(f)
	assert.NilError(t, err)
	assert.Equal(t, off, int64(40)*gptLBASectorSize)
}
