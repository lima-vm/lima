// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package apfs

import (
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"os"
	"strings"
)

var le = binary.LittleEndian

// Chown sets the owner and group of files on an unmounted APFS disk image.
// volumeRole selects the target volume (e.g., VolRoleData).
// Paths are relative to the volume root (e.g., "Library/LaunchDaemons/foo.plist").
func Chown(diskPath string, volumeRole uint16, uid, gid uint32, paths ...string) error {
	c, err := openContainer(diskPath)
	if err != nil {
		return err
	}
	defer c.close()

	vol, err := c.findVolume(volumeRole)
	if err != nil {
		return err
	}

	fsRootPhys, err := c.omapLookup(vol.omapTreeAddr, vol.rootTreeOID, vol.latestXID)
	if err != nil {
		return fmt.Errorf("resolving filesystem root tree OID %d: %w", vol.rootTreeOID, err)
	}

	for _, path := range paths {
		inodeNum, err := c.resolvePath(fsRootPhys, vol.omapTreeAddr, vol.latestXID, path)
		if err != nil {
			return fmt.Errorf("resolving path %q: %w", path, err)
		}
		if err := c.chownInode(fsRootPhys, vol.omapTreeAddr, vol.latestXID, inodeNum, uid, gid); err != nil {
			return fmt.Errorf("chown inode %d (%q): %w", inodeNum, path, err)
		}
	}
	return nil
}

// container holds the open disk image file, the byte offset where
// the APFS container starts (nonzero for GPT-partitioned disks),
// and the APFS block size.
type container struct {
	f          *os.File
	baseOffset int64 // byte offset of APFS container within file
	blockSize  uint32
}

// volumeInfo holds resolved volume information.
type volumeInfo struct {
	omapTreeAddr uint64 // physical address of volume omap B-tree root
	rootTreeOID  uint64 // virtual OID of filesystem B-tree root
	latestXID    uint64 // transaction ID of the volume superblock
}

func openContainer(path string) (*container, error) {
	f, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		return nil, fmt.Errorf("open disk: %w", err)
	}
	c := &container{f: f}

	hdr := make([]byte, 4096) // minimum APFS block size
	if _, err := f.ReadAt(hdr, 0); err != nil {
		f.Close()
		return nil, fmt.Errorf("read block 0: %w", err)
	}

	if le.Uint32(hdr[nxMagicOff:]) == nxMagic {
		// Raw APFS container (no partition table).
		c.blockSize = le.Uint32(hdr[nxBlockSizeOff:])
	} else {
		// Look for a GPT partition table and find the APFS partition.
		offset, err := findAPFSPartitionGPT(f)
		if err != nil {
			f.Close()
			return nil, fmt.Errorf("finding APFS partition: %w", err)
		}
		c.baseOffset = offset
		if _, err := f.ReadAt(hdr, offset); err != nil {
			f.Close()
			return nil, fmt.Errorf("read APFS superblock at offset %d: %w", offset, err)
		}
		if le.Uint32(hdr[nxMagicOff:]) != nxMagic {
			f.Close()
			return nil, fmt.Errorf("APFS partition at offset %d has bad magic", offset)
		}
		c.blockSize = le.Uint32(hdr[nxBlockSizeOff:])
	}

	if c.blockSize < 4096 {
		f.Close()
		return nil, fmt.Errorf("invalid block size %d", c.blockSize)
	}
	return c, nil
}

// GPT constants.
const (
	gptHeaderSignature = "EFI PART"
	gptLBASectorSize   = 512
)

// apfsPartTypeGUID is the APFS Container partition type GUID as stored
// on disk (mixed-endian encoding per GPT spec).
// 7C3457EF-0000-11AA-AA11-00306543ECAC.
var apfsPartTypeGUID = [16]byte{
	0xEF, 0x57, 0x34, 0x7C, // time_low (LE)
	0x00, 0x00, // time_mid (LE)
	0xAA, 0x11, // time_hi_and_version (LE)
	0xAA, 0x11, // clock_seq
	0x00, 0x30, 0x65, 0x43, 0xEC, 0xAC, // node
}

// findAPFSPartitionGPT reads a GPT partition table and returns the
// byte offset of the first APFS Container partition.
func findAPFSPartitionGPT(f *os.File) (int64, error) {
	// Read GPT header at LBA 1.
	gptHdr := make([]byte, gptLBASectorSize)
	if _, err := f.ReadAt(gptHdr, gptLBASectorSize); err != nil {
		return 0, fmt.Errorf("reading GPT header: %w", err)
	}
	if string(gptHdr[0:8]) != gptHeaderSignature {
		return 0, fmt.Errorf("no GPT header found (expected %q)", gptHeaderSignature)
	}

	partEntryLBA := le.Uint64(gptHdr[72:])
	numEntries := le.Uint32(gptHdr[80:])
	entrySize := le.Uint32(gptHdr[84:])

	entryBuf := make([]byte, entrySize)
	for i := range numEntries {
		off := int64(partEntryLBA)*gptLBASectorSize + int64(i)*int64(entrySize)
		if _, err := f.ReadAt(entryBuf, off); err != nil {
			return 0, fmt.Errorf("reading GPT entry %d: %w", i, err)
		}

		var typeGUID [16]byte
		copy(typeGUID[:], entryBuf[0:16])
		if typeGUID == apfsPartTypeGUID {
			firstLBA := le.Uint64(entryBuf[32:])
			return int64(firstLBA) * gptLBASectorSize, nil
		}

		// Stop at empty entries.
		allZero := true
		for _, b := range typeGUID {
			if b != 0 {
				allZero = false
				break
			}
		}
		if allZero {
			break
		}
	}
	return 0, errors.New("no APFS partition found in GPT")
}

func (c *container) close() {
	c.f.Close()
}

func (c *container) readBlock(addr uint64) ([]byte, error) {
	buf := make([]byte, c.blockSize)
	_, err := c.f.ReadAt(buf, c.baseOffset+int64(addr)*int64(c.blockSize))
	if err != nil {
		return nil, fmt.Errorf("read block %d: %w", addr, err)
	}
	return buf, nil
}

func (c *container) writeBlock(addr uint64, data []byte) error {
	_, err := c.f.WriteAt(data, c.baseOffset+int64(addr)*int64(c.blockSize))
	if err != nil {
		return fmt.Errorf("write block %d: %w", addr, err)
	}
	return nil
}

// latestSuperblock scans the checkpoint descriptor area for the
// superblock with the highest valid transaction ID.
func (c *container) latestSuperblock() ([]byte, error) {
	// Read block 0 at full block size to get checkpoint descriptor area info.
	block0, err := c.readBlock(0)
	if err != nil {
		return nil, err
	}
	if err := verifyChecksum(block0); err != nil {
		return nil, fmt.Errorf("block 0 checksum: %w", err)
	}

	descBase := le.Uint64(block0[nxXPDescBaseOff:])
	descBlocks := le.Uint32(block0[nxXPDescBlocksOff:]) & nxXPDescBlocksMask

	var bestBlock []byte
	var bestXID uint64

	for i := range descBlocks {
		blk, err := c.readBlock(descBase + uint64(i))
		if err != nil {
			continue
		}
		if verifyChecksum(blk) != nil {
			continue
		}
		oType := le.Uint32(blk[objTypeOff:]) & objTypeMask
		if oType != objectTypeNXSuperblock {
			continue
		}
		if le.Uint32(blk[nxMagicOff:]) != nxMagic {
			continue
		}
		xid := le.Uint64(blk[objXIDOff:])
		if xid > bestXID {
			bestXID = xid
			bestBlock = blk
		}
	}
	if bestBlock == nil {
		return nil, errors.New("no valid container superblock found in checkpoint area")
	}
	return bestBlock, nil
}

// findVolume locates the volume with the given role.
func (c *container) findVolume(role uint16) (*volumeInfo, error) {
	sb, err := c.latestSuperblock()
	if err != nil {
		return nil, fmt.Errorf("reading container superblock: %w", err)
	}

	// Read the container omap to resolve volume virtual OIDs.
	containerOmapAddr := le.Uint64(sb[nxOmapOIDOff:])
	containerOmap, err := c.readBlock(containerOmapAddr)
	if err != nil {
		return nil, fmt.Errorf("reading container omap at %d: %w", containerOmapAddr, err)
	}
	containerOmapTreeAddr := le.Uint64(containerOmap[omapTreeOIDOff:])

	containerXID := le.Uint64(sb[objXIDOff:])

	for i := range nxMaxFileSystems {
		off := nxFSOIDOff + i*8
		volOID := le.Uint64(sb[off:])
		if volOID == 0 {
			continue
		}
		// Resolve virtual OID through container omap.
		volPhysAddr, err := c.omapLookup(containerOmapTreeAddr, volOID, containerXID)
		if err != nil {
			continue // skip volumes we can't resolve
		}
		volBlock, err := c.readBlock(volPhysAddr)
		if err != nil {
			continue
		}
		if verifyChecksum(volBlock) != nil {
			continue
		}
		if le.Uint32(volBlock[apfsMagicOff:]) != apfsMagic {
			continue
		}
		volRole := le.Uint16(volBlock[apfsRoleOff:])
		if volRole == role {
			omapAddr := le.Uint64(volBlock[apfsOmapOIDOff:])
			omapBlock, err := c.readBlock(omapAddr)
			if err != nil {
				return nil, fmt.Errorf("reading volume omap: %w", err)
			}
			return &volumeInfo{
				omapTreeAddr: le.Uint64(omapBlock[omapTreeOIDOff:]),
				rootTreeOID:  le.Uint64(volBlock[apfsRootTreeOIDOff:]),
				latestXID:    le.Uint64(volBlock[objXIDOff:]),
			}, nil
		}
	}
	return nil, fmt.Errorf("no volume with role %#x found", role)
}

// omapLookup searches the omap B-tree for a virtual OID, returning
// the physical address from the entry with the highest xid <= maxXID.
func (c *container) omapLookup(omapTreeAddr, oid, maxXID uint64) (uint64, error) {
	blk, err := c.readBlock(omapTreeAddr)
	if err != nil {
		return 0, err
	}

	for {
		if verifyChecksum(blk) != nil {
			return 0, errors.New("omap node checksum failed")
		}
		if err := verifyBTreeNodeType(blk); err != nil {
			return 0, fmt.Errorf("omap node: %w", err)
		}
		flags := le.Uint16(blk[btnFlagsOff:])
		nkeys := le.Uint32(blk[btnNKeysOff:])
		tspOff := le.Uint16(blk[btnTableSpaceOff:])
		tspLen := le.Uint16(blk[btnTableSpaceOff+2:])

		tocStart := btnDataOff + uint32(tspOff)
		keyAreaStart := tocStart + uint32(tspLen)

		isLeaf := flags&btnodeLeaf != 0
		isFixedKV := flags&btnodeFixedKVSize != 0
		isRoot := flags&btnodeRoot != 0

		valueAreaEnd := c.blockSize
		if isRoot {
			valueAreaEnd -= btreeInfoSize
		}

		if isLeaf {
			// Search for matching oid with highest xid <= maxXID.
			var bestPhysAddr uint64
			var bestXID uint64
			found := false

			for i := range nkeys {
				kOff, vOff := c.readTocEntry(blk, tocStart, i, isFixedKV)
				keyStart := keyAreaStart + kOff
				entryOID := le.Uint64(blk[keyStart:])
				entryXID := le.Uint64(blk[keyStart+8:])

				if entryOID == oid && entryXID <= maxXID {
					// Value: ov_flags(4) + ov_size(4) + ov_paddr(8).
					valStart := valueAreaEnd - vOff
					physAddr := le.Uint64(blk[valStart+8:])
					if !found || entryXID > bestXID {
						bestPhysAddr = physAddr
						bestXID = entryXID
						found = true
					}
				}
			}
			if !found {
				return 0, fmt.Errorf("omap entry for OID %d not found", oid)
			}
			return bestPhysAddr, nil
		}

		// Internal node: find the last key <= (oid, maxXID) and descend.
		childIdx := uint32(0)
		for i := range nkeys {
			kOff, _ := c.readTocEntry(blk, tocStart, i, isFixedKV)
			keyStart := keyAreaStart + kOff
			entryOID := le.Uint64(blk[keyStart:])
			entryXID := le.Uint64(blk[keyStart+8:])

			cmp := compareOmapKey(entryOID, entryXID, oid, maxXID)
			if cmp <= 0 {
				childIdx = i
			} else {
				break
			}
		}

		// Read child pointer (physical address, 8 bytes).
		_, vOff := c.readTocEntry(blk, tocStart, childIdx, isFixedKV)
		childAddr := le.Uint64(blk[valueAreaEnd-vOff:])

		blk, err = c.readBlock(childAddr)
		if err != nil {
			return 0, fmt.Errorf("reading omap child node: %w", err)
		}
	}
}

// readTocEntry reads the key and value offsets from a ToC entry.
// For fixed-KV nodes (kvoff_t): 4 bytes per entry (k_off u16, v_off u16).
// For variable-KV nodes (kvloc_t): 8 bytes per entry (k.off u16, k.len u16, v.off u16, v.len u16).
// Returns keyOffset and valueOffset (both relative to their respective areas).
func (c *container) readTocEntry(blk []byte, tocStart, index uint32, fixedKV bool) (keyOff, valOff uint32) {
	if fixedKV {
		off := tocStart + index*4
		return uint32(le.Uint16(blk[off:])), uint32(le.Uint16(blk[off+2:]))
	}
	off := tocStart + index*8
	return uint32(le.Uint16(blk[off:])), uint32(le.Uint16(blk[off+4:]))
}

// verifyBTreeNodeType checks that a block's object type indicates a
// B-tree node: OBJECT_TYPE_BTREE (0x02, root) or OBJECT_TYPE_BTREE_NODE
// (0x03, non-root).
func verifyBTreeNodeType(blk []byte) error {
	oType := le.Uint32(blk[objTypeOff:]) & objTypeMask
	if oType != 0x02 && oType != 0x03 {
		return fmt.Errorf("expected B-tree node type (2 or 3), got %#x", oType)
	}
	return nil
}

func compareOmapKey(oid1, xid1, oid2, xid2 uint64) int {
	if oid1 < oid2 {
		return -1
	}
	if oid1 > oid2 {
		return 1
	}
	if xid1 < xid2 {
		return -1
	}
	if xid1 > xid2 {
		return 1
	}
	return 0
}

// compareFSKeyHeader compares two filesystem B-tree key headers.
// APFS sorts filesystem keys by (obj_id, type), not by the raw uint64.
func compareFSKeyHeader(a, b uint64) int {
	aID := a & objIDMask
	bID := b & objIDMask
	if aID < bID {
		return -1
	}
	if aID > bID {
		return 1
	}
	aType := a >> objTypeShift
	bType := b >> objTypeShift
	if aType < bType {
		return -1
	}
	if aType > bType {
		return 1
	}
	return 0
}

var crc32cTable = crc32.MakeTable(crc32.Castagnoli)

// drecNameHash computes the 22-bit directory record name hash as stored
// in the upper bits of j_drec_hashed_key_t.name_len_and_hash.
//
// Per the Apple APFS Reference: NFD-normalize the name, encode as UTF-32LE
// (without null terminator), compute CRC-32C, complement, keep low 22 bits.
// The complement cancels with the CRC's standard finalization XOR, so we
// use ^crc32.Checksum to get the raw CRC.
func drecNameHash(name string) uint32 {
	runes := []rune(strings.ToLower(name))
	buf := make([]byte, len(runes)*4)
	for i, r := range runes {
		le.PutUint32(buf[i*4:], uint32(r))
	}
	return ^crc32.Checksum(buf, crc32cTable) & 0x3FFFFF
}

// compareDrecKey compares a directory record key from a B-tree node
// against a target (parentCNID, name). Returns <0, 0, or >0.
func (c *container) compareDrecKey(blk []byte, keyStart uint32, targetKeyHeader uint64, targetName string, targetHash uint32) int {
	keyHeader := le.Uint64(blk[keyStart:])
	cmp := compareFSKeyHeader(keyHeader, targetKeyHeader)
	if cmp != 0 {
		return cmp
	}
	// Headers match (same parent CNID and type DIR_REC).
	// Compare name hash for hashed keys, or name directly for non-hashed.
	val32 := le.Uint32(blk[keyStart+8:])
	if val32&0xFFFFFC00 != 0 {
		// Hashed key: compare hash values (upper 22 bits).
		entryHash := (val32 >> 10) & 0x3FFFFF
		if entryHash < targetHash {
			return -1
		}
		if entryHash > targetHash {
			return 1
		}
		// Hash collision: compare actual names.
		entryName := c.readDrecName(blk, keyStart+8)
		return strings.Compare(strings.ToLower(entryName), strings.ToLower(targetName))
	}
	// Non-hashed key: compare names directly.
	entryName := c.readDrecName(blk, keyStart+8)
	return strings.Compare(entryName, targetName)
}

// resolvePath walks path components from the volume root, returning
// the inode number of the final path element.
func (c *container) resolvePath(fsRootPhys, omapTreeAddr, maxXID uint64, path string) (uint64, error) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	cnid := uint64(rootDirInodeNum)

	for _, name := range parts {
		if name == "" {
			continue
		}
		fileID, err := c.lookupDirEntry(fsRootPhys, omapTreeAddr, maxXID, cnid, name)
		if err != nil {
			return 0, fmt.Errorf("looking up %q in directory (cnid %d): %w", name, cnid, err)
		}
		cnid = fileID
	}
	return cnid, nil
}

// lookupDirEntry searches the filesystem B-tree for a directory record
// matching parentCNID and name, returning the file_id from j_drec_val_t.
func (c *container) lookupDirEntry(fsRootPhys, omapTreeAddr, maxXID, parentCNID uint64, name string) (uint64, error) {
	targetKeyHeader := (uint64(apfsTypeDirRec) << objTypeShift) | (parentCNID & objIDMask)
	targetHash := drecNameHash(name)

	blk, err := c.readBlock(fsRootPhys)
	if err != nil {
		return 0, err
	}

	for {
		if verifyChecksum(blk) != nil {
			return 0, errors.New("filesystem B-tree node checksum failed")
		}
		if err := verifyBTreeNodeType(blk); err != nil {
			return 0, fmt.Errorf("filesystem B-tree node: %w", err)
		}
		flags := le.Uint16(blk[btnFlagsOff:])
		nkeys := le.Uint32(blk[btnNKeysOff:])
		tspOff := le.Uint16(blk[btnTableSpaceOff:])
		tspLen := le.Uint16(blk[btnTableSpaceOff+2:])

		tocStart := btnDataOff + uint32(tspOff)
		keyAreaStart := tocStart + uint32(tspLen)

		isLeaf := flags&btnodeLeaf != 0
		isFixedKV := flags&btnodeFixedKVSize != 0
		isRoot := flags&btnodeRoot != 0

		valueAreaEnd := c.blockSize
		if isRoot {
			valueAreaEnd -= btreeInfoSize
		}

		if isLeaf {
			for i := range nkeys {
				kOff, vOff := c.readTocEntry(blk, tocStart, i, isFixedKV)
				keyStart := keyAreaStart + kOff
				keyHeader := le.Uint64(blk[keyStart:])

				if keyHeader != targetKeyHeader {
					continue
				}

				// Parse the directory record key name.
				entryName := c.readDrecName(blk, keyStart+8)
				if entryName != name {
					continue
				}

				// Read file_id from j_drec_val_t (first 8 bytes of value).
				valStart := valueAreaEnd - vOff
				return le.Uint64(blk[valStart:]), nil
			}
			return 0, fmt.Errorf("directory entry %q not found", name)
		}

		// Internal node: find the child to descend into.
		// The last key <= our target determines the child.
		// For DIR_REC keys with matching headers, we also compare the
		// name hash to find the correct subtree.
		childIdx := uint32(0)
		for i := range nkeys {
			kOff, _ := c.readTocEntry(blk, tocStart, i, isFixedKV)
			keyStart := keyAreaStart + kOff
			cmp := c.compareDrecKey(blk, keyStart, targetKeyHeader, name, targetHash)
			if cmp <= 0 {
				childIdx = i
			} else {
				break
			}
		}

		// Read child OID (virtual) and resolve through omap.
		_, vOff := c.readTocEntry(blk, tocStart, childIdx, isFixedKV)
		childOID := le.Uint64(blk[valueAreaEnd-vOff:])
		childPhys, err := c.omapLookup(omapTreeAddr, childOID, maxXID)
		if err != nil {
			return 0, fmt.Errorf("resolving child OID %d: %w", childOID, err)
		}
		blk, err = c.readBlock(childPhys)
		if err != nil {
			return 0, err
		}
	}
}

// readDrecName reads a directory record name from the key data
// starting at nameFieldOff. Handles both hashed (j_drec_hashed_key_t,
// 4-byte name_len_and_hash) and non-hashed (j_drec_key_t, 2-byte
// name_len) formats.
func (c *container) readDrecName(blk []byte, nameFieldOff uint32) string {
	// Heuristic: if the 4-byte field at nameFieldOff has its upper
	// bits set (hash), it's a hashed key. For non-hashed keys, the
	// 2-byte name_len is followed by the name bytes.
	//
	// In j_drec_hashed_key_t: name_len_and_hash (uint32) where
	// lower 10 bits = name length including null terminator.
	// In j_drec_key_t: name_len (uint16) = name length including
	// null terminator.
	//
	// We distinguish them by checking: if the upper 22 bits of the
	// first uint32 are nonzero, it's hashed.
	val32 := le.Uint32(blk[nameFieldOff:])
	if val32&0xFFFFFC00 != 0 {
		// Hashed: lower 10 bits = length (including null terminator).
		nameLen := int(val32 & 0x3FF)
		nameStart := nameFieldOff + 4
		if nameLen > 1 {
			return string(blk[nameStart : nameStart+uint32(nameLen-1)])
		}
		return ""
	}
	// Non-hashed: 2-byte name_len.
	nameLen := int(le.Uint16(blk[nameFieldOff:]))
	nameStart := nameFieldOff + 2
	if nameLen > 1 {
		return string(blk[nameStart : nameStart+uint32(nameLen-1)])
	}
	return ""
}

// chownInode finds an inode in the filesystem B-tree and modifies
// its owner and group fields.
func (c *container) chownInode(fsRootPhys, omapTreeAddr, maxXID, inodeNum uint64, uid, gid uint32) error {
	targetKeyHeader := (uint64(apfsTypeInode) << objTypeShift) | (inodeNum & objIDMask)

	blkAddr := fsRootPhys
	blk, err := c.readBlock(blkAddr)
	if err != nil {
		return err
	}

	// Walk down to the leaf containing the inode.
	for {
		if verifyChecksum(blk) != nil {
			return errors.New("filesystem B-tree node checksum failed")
		}
		if err := verifyBTreeNodeType(blk); err != nil {
			return fmt.Errorf("filesystem B-tree node: %w", err)
		}
		flags := le.Uint16(blk[btnFlagsOff:])
		nkeys := le.Uint32(blk[btnNKeysOff:])
		tspOff := le.Uint16(blk[btnTableSpaceOff:])
		tspLen := le.Uint16(blk[btnTableSpaceOff+2:])

		tocStart := btnDataOff + uint32(tspOff)
		keyAreaStart := tocStart + uint32(tspLen)

		isLeaf := flags&btnodeLeaf != 0
		isFixedKV := flags&btnodeFixedKVSize != 0
		isRoot := flags&btnodeRoot != 0

		valueAreaEnd := c.blockSize
		if isRoot {
			valueAreaEnd -= btreeInfoSize
		}

		if isLeaf {
			for i := range nkeys {
				kOff, vOff := c.readTocEntry(blk, tocStart, i, isFixedKV)
				keyStart := keyAreaStart + kOff
				keyHeader := le.Uint64(blk[keyStart:])

				if keyHeader != targetKeyHeader {
					continue
				}

				// Found the inode. Validate before writing.
				valStart := valueAreaEnd - vOff

				if valStart+inodeGroupOff+4 > c.blockSize {
					return fmt.Errorf("inode %d value exceeds block boundary", inodeNum)
				}

				if privateID := le.Uint64(blk[valStart+inodePrivateIDOff:]); privateID != inodeNum {
					return fmt.Errorf("inode %d has mismatched private_id %d", inodeNum, privateID)
				}

				currentUID := le.Uint32(blk[valStart+inodeOwnerOff:])
				currentGID := le.Uint32(blk[valStart+inodeGroupOff:])
				if currentUID != noownersPlaceholderID || currentGID != noownersPlaceholderID {
					return fmt.Errorf("inode %d has ownership %d:%d, expected %d:%d from noowners mount",
						inodeNum, currentUID, currentGID, noownersPlaceholderID, noownersPlaceholderID)
				}

				le.PutUint32(blk[valStart+inodeOwnerOff:], uid)
				le.PutUint32(blk[valStart+inodeGroupOff:], gid)
				updateChecksum(blk)
				return c.writeBlock(blkAddr, blk)
			}
			return fmt.Errorf("inode %d not found in filesystem B-tree", inodeNum)
		}

		// Internal node: descend.
		childIdx := uint32(0)
		for i := range nkeys {
			kOff, _ := c.readTocEntry(blk, tocStart, i, isFixedKV)
			keyStart := keyAreaStart + kOff
			keyHeader := le.Uint64(blk[keyStart:])

			if compareFSKeyHeader(keyHeader, targetKeyHeader) <= 0 {
				childIdx = i
			} else {
				break
			}
		}

		_, vOff := c.readTocEntry(blk, tocStart, childIdx, isFixedKV)
		childOID := le.Uint64(blk[valueAreaEnd-vOff:])
		childPhys, err := c.omapLookup(omapTreeAddr, childOID, maxXID)
		if err != nil {
			return fmt.Errorf("resolving child OID %d: %w", childOID, err)
		}
		blkAddr = childPhys
		blk, err = c.readBlock(blkAddr)
		if err != nil {
			return err
		}
	}
}
