// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package limayaml

// DefaultDiskFSType is the filesystem an additional disk is formatted with when
// `additionalDisks[].fsType` is not set. It must match the default applied by
// boot.Linux/05-lima-disks.sh.
const DefaultDiskFSType = "ext4"

// diskLabelPrefix is prepended to `additionalDisks[].name` to form the
// filesystem and GPT partition labels of the disk.
const diskLabelPrefix = "lima-"

// DiskFSLabel returns the filesystem label that an additional disk named name
// is formatted with, before any filesystem-specific truncation.
func DiskFSLabel(name string) string {
	return diskLabelPrefix + name
}

// FSLabelMaxLen returns the maximum filesystem label length, in bytes, accepted
// by fsType, or 0 if Lima does not know that limit.
//
// mkfs enforces the limit differently depending on the filesystem: mkfs.ext4
// truncates an over-long label silently, while mkfs.xfs rejects it outright.
// Lima therefore clamps the label in boot.Linux/05-lima-disks.sh, and the
// limits here must stay in sync with the ones in that script. For an fsType not
// listed here, the limit (and even whether mkfs.$fsType accepts a label at all)
// is unknown, so the boot script omits the label rather than guess.
//
// This affects the label only. Lima does not identify a disk by its label, so a
// name too long to fit is not an error.
func FSLabelMaxLen(fsType string) int {
	switch fsType {
	case "ext2", "ext3", "ext4":
		return 16
	case "xfs":
		return 12
	case "btrfs":
		return 255
	case "swap":
		// mkswap writes the label into a 16-byte field that includes the
		// terminating NUL.
		return 15
	default:
		return 0
	}
}
