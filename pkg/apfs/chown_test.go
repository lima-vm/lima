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

// TestParseBTreeNodeRejectsBadHeader checks that a B-tree node header whose
// table of contents does not fit in the block is rejected rather than letting
// the per-entry offset reads index out of bounds.
func TestParseBTreeNodeRejectsBadHeader(t *testing.T) {
	c := &container{blockSize: 4096}
	node := func(flags uint16, nkeys uint32, tspOff, tspLen uint16) []byte {
		blk := make([]byte, c.blockSize)
		le.PutUint32(blk[objTypeOff:], objectTypeBTreeNode)
		le.PutUint16(blk[btnFlagsOff:], flags)
		le.PutUint32(blk[btnNKeysOff:], nkeys)
		le.PutUint16(blk[btnTableSpaceOff:], tspOff)
		le.PutUint16(blk[btnTableSpaceOff+2:], tspLen)
		return blk
	}
	for _, tc := range []struct {
		name   string
		flags  uint16
		nkeys  uint32
		tspOff uint16
		tspLen uint16
		ok     bool
	}{
		{"valid-fixed-kv", btnodeLeaf | btnodeFixedKVSize, 2, 0, 8, true},
		{"toc-past-block", btnodeLeaf | btnodeFixedKVSize, 1, 5000, 8, false},
		{"key-area-past-block", btnodeLeaf | btnodeFixedKVSize, 1, 0, 5000, false},
		{"nkeys-exceed-toc-fixed", btnodeLeaf | btnodeFixedKVSize, 1000, 0, 8, false},
		{"nkeys-exceed-toc-var", btnodeLeaf, 1000, 0, 8, false},
		{"root-smaller-than-info", btnodeRoot | btnodeLeaf | btnodeFixedKVSize, 0, 0, 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := c.parseBTreeNode(node(tc.flags, tc.nkeys, tc.tspOff, tc.tspLen))
			if tc.ok {
				assert.NilError(t, err)
				return
			}
			assert.ErrorContains(t, err, "out of range")
		})
	}
}

// TestBTreeNodeOffsetsRejectOutOfRange checks the key/value accessors refuse
// offsets that fall outside the node, including a value offset large enough to
// underflow valueAreaEnd-vOff (the case the old chownInode guard, computed in
// wrapping uint32 arithmetic, could let slip through).
func TestBTreeNodeOffsetsRejectOutOfRange(t *testing.T) {
	n := &btreeNode{blk: make([]byte, 4096), keyAreaStart: 100, valueAreaEnd: 200}

	if _, ok := n.keyOffset(10, 8); !ok {
		t.Error("keyOffset rejected an in-range key")
	}
	if _, ok := n.keyOffset(500, 8); ok {
		t.Error("keyOffset accepted a key past the value area")
	}
	if off, ok := n.valueOffset(50, 8); !ok || off != 150 {
		t.Errorf("valueOffset(50,8) = %d, %v; want 150, true", off, ok)
	}
	if _, ok := n.valueOffset(5000, 8); ok {
		t.Error("valueOffset accepted a value offset that underflows valueAreaEnd-vOff")
	}
	if _, ok := n.valueOffset(150, 8); ok {
		t.Error("valueOffset accepted a value that overlaps the key area")
	}
}

// TestOmapLookupRejectsCraftedNode drives a crafted omap leaf (whose checksum
// is valid, so the parser reaches the entry reads) through omapLookup and
// asserts it returns an error instead of panicking on an out-of-range value
// offset.
func TestOmapLookupRejectsCraftedNode(t *testing.T) {
	const (
		blockSize = 4096
		oid       = uint64(0x1234)
	)
	blk := make([]byte, blockSize)
	le.PutUint32(blk[objTypeOff:], objectTypeBTreeNode) // non-root B-tree node
	le.PutUint16(blk[btnFlagsOff:], btnodeLeaf|btnodeFixedKVSize)
	le.PutUint32(blk[btnNKeysOff:], 1)
	le.PutUint16(blk[btnTableSpaceOff:], 0)
	le.PutUint16(blk[btnTableSpaceOff+2:], 8) // toc: room for two kvoff_t entries

	tocStart := uint32(btnDataOff)
	keyAreaStart := tocStart + 8
	le.PutUint16(blk[tocStart:], 0)       // entry0 k_off -> keyStart == keyAreaStart
	le.PutUint16(blk[tocStart+2:], 60000) // entry0 v_off: valueAreaEnd(4096) - 60000 underflows
	le.PutUint64(blk[keyAreaStart:], oid) // ov_key.oid
	le.PutUint64(blk[keyAreaStart+8:], 0) // ov_key.xid (<= maxXID)
	updateChecksum(blk)

	path := filepath.Join(t.TempDir(), "omap.img")
	assert.NilError(t, os.WriteFile(path, blk, 0o600))
	f, err := os.Open(path)
	assert.NilError(t, err)
	t.Cleanup(func() { _ = f.Close() })

	c := &container{f: f, blockSize: blockSize}
	_, err = c.omapLookup(0, oid, 100)
	assert.ErrorContains(t, err, "out of range")
}

// TestOmapLookupRejectsEmptyInternalNode drives an internal (non-leaf) node
// with no keys through omapLookup. Each walk's internal-node branch reads child
// entry 0 even when the key loop never runs, so an empty table of contents sat
// at the end of the block would index past it; parseBTreeNode must reject the
// node instead.
func TestOmapLookupRejectsEmptyInternalNode(t *testing.T) {
	const (
		blockSize = 4096
		oid       = uint64(0x1234)
	)
	blk := make([]byte, blockSize)
	le.PutUint32(blk[objTypeOff:], objectTypeBTreeNode) // non-root B-tree node
	le.PutUint16(blk[btnFlagsOff:], btnodeFixedKVSize)  // internal (no btnodeLeaf)
	le.PutUint32(blk[btnNKeysOff:], 0)
	le.PutUint16(blk[btnTableSpaceOff:], blockSize-btnDataOff) // tocStart at end of block
	le.PutUint16(blk[btnTableSpaceOff+2:], 0)
	updateChecksum(blk)

	path := filepath.Join(t.TempDir(), "omap.img")
	assert.NilError(t, os.WriteFile(path, blk, 0o600))
	f, err := os.Open(path)
	assert.NilError(t, err)
	t.Cleanup(func() { _ = f.Close() })

	c := &container{f: f, blockSize: blockSize}
	_, err = c.omapLookup(0, oid, 100)
	assert.ErrorContains(t, err, "internal node has no keys")
}
