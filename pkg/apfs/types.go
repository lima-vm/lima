// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

// Package apfs provides minimal APFS disk image manipulation.
// It reads on-disk structures at known byte offsets per the Apple APFS
// Reference and modifies inode UID/GID fields directly, bypassing
// the kernel VFS.
package apfs

// Magic numbers.
const (
	nxMagic   = 0x4253584E // "NXSB"
	apfsMagic = 0x42535041 // "APSB"
)

// Object types (lower 16 bits of o_type).
const (
	objectTypeNXSuperblock    = 0x01
	objectTypeCheckpointMap   = 0x0C
	objectTypeOmap            = 0x0B
	objectTypeBTreeNode       = 0x02
	objectTypeFS              = 0x0D
	objectTypeFSTree          = 0x0E
	objectTypeBlockRefTree    = 0x0F
	objectTypeOmapSnapshot    = 0x10
	objectTypeNXReaperFS      = 0x12
	objectTypeNXReaperListKey = 0x13
)

// Object storage classes (upper 16 bits of o_type, shifted).
const (
	objPhysical  = 0x00000000
	objEphemeral = 0x80000000
	objVirtual   = 0x00000000
	objTypeMask  = 0x0000FFFF
	objFlagsMask = 0xFFFF0000
)

// B-tree node flags.
const (
	btnodeRoot        = 0x0001
	btnodeLeaf        = 0x0002
	btnodeFixedKVSize = 0x0004
)

// Filesystem object types (upper 4 bits of j_key_t.obj_id_and_type).
const (
	apfsTypeInode  = 3
	apfsTypeDirRec = 9
)

// Filesystem key masks.
const (
	objIDMask    = 0x0FFFFFFFFFFFFFFF
	objTypeShift = 60
)

// Well-known inode numbers.
const (
	rootDirInodeNum = 2
)

// noownersPlaceholderID is the on-disk UID and GID that APFS stores
// for files on volumes mounted with the noowners option.
const noownersPlaceholderID = 99

// VolRoleData is the APFS volume role for "Data" volumes.
const VolRoleData = 0x0040 // APFS_VOL_ROLE_DATA (shifted representation: (1 << 2) << 4)

// Container superblock (nx_superblock_t) field offsets from block start.
const (
	nxMagicOff         = 32  // uint32
	nxBlockSizeOff     = 36  // uint32
	nxXPDescBlocksOff  = 104 // uint32 (mask 0x7FFFFFFF for count)
	nxXPDescBaseOff    = 112 // uint64 (physical block address)
	nxXPDescIndexOff   = 136 // uint32
	nxXPDescLenOff     = 140 // uint32
	nxOmapOIDOff       = 160 // uint64 (physical)
	nxFSOIDOff         = 184 // uint64[100] (virtual OIDs)
	nxMaxFileSystems   = 100
	nxXPDescBlocksMask = 0x7FFFFFFF
)

// Object header (obj_phys_t) field offsets.
const (
	objChecksumOff = 0  // uint64
	objOIDOff      = 8  // uint64
	objXIDOff      = 16 // uint64
	objTypeOff     = 24 // uint32
	objSubtypeOff  = 28 // uint32
	objHeaderSize  = 32
)

// Volume superblock (apfs_superblock_t) field offsets from block start.
const (
	apfsMagicOff       = 32  // uint32
	apfsOmapOIDOff     = 128 // uint64 (physical)
	apfsRootTreeOIDOff = 136 // uint64 (virtual)
	apfsVolNameOff     = 704 // 256 bytes UTF-8
	apfsRoleOff        = 964 // uint16
)

// Object map (omap_phys_t) field offsets from block start.
const (
	omapTreeOIDOff = 48 // uint64 (physical)
)

// B-tree node (btree_node_phys_t) field offsets from block start.
const (
	btnFlagsOff      = 32 // uint16
	btnLevelOff      = 34 // uint16
	btnNKeysOff      = 36 // uint32
	btnTableSpaceOff = 40 // nloc_t: off uint16 + len uint16
	btnDataOff       = 56 // start of btn_data[]
)

// btree_info_t size (sits at end of root node block).
const btreeInfoSize = 40

// j_inode_val_t field offsets (from start of value data).
const (
	inodeParentIDOff  = 0  // uint64
	inodePrivateIDOff = 8  // uint64
	inodeOwnerOff     = 72 // uint32
	inodeGroupOff     = 76 // uint32
)
