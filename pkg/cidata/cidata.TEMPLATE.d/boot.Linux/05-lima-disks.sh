#!/bin/bash

# SPDX-FileCopyrightText: Copyright The Lima Authors
# SPDX-License-Identifier: Apache-2.0

set -eux -o pipefail

test "$LIMA_CIDATA_DISKS" -gt 0 || exit 0

get_disk_var() {
	diskvarname="LIMA_CIDATA_DISK_${1}_${2}"
	eval echo \$"$diskvarname"
}

# partition_needs_setup PARTITION succeeds when PARTITION carries no filesystem
# (or does not exist yet), i.e. when first-time setup must partition and format
# the disk.
#
# The TYPE tag is queried specifically, not just whether blkid recognizes the
# partition at all: on GPT, blkid reports PARTUUID/PARTLABEL for a bare
# partition-table entry before it ever has a filesystem, so testing for any
# blkid output would treat a boot interrupted between sfdisk and mkfs as
# already formatted -- and then fail at mount/swapon on every later boot.
#
# The output is tested, not the exit status: util-linux blkid exits 0 for such a
# bare entry even with -s TYPE, and BusyBox blkid (Alpine) ignores -o and -s and
# always exits 0, but prints a line only when it finds a filesystem.
#
# It fails closed: when blkid is missing, or exits with anything other than 0 or
# 2 ("nothing found"), the partition is reported as not needing setup, so a disk
# whose state cannot be determined is never reformatted. Its mount then fails
# instead, which boot.sh reports as a warning.
partition_needs_setup() {
	if ! command -v blkid >/dev/null 2>&1; then
		echo >&2 "WARNING: blkid not found; not formatting $1, as it may already hold data"
		return 1
	fi
	blkid_status=0
	blkid_fstype="$(blkid -o value -s TYPE "$1" 2>/dev/null)" || blkid_status=$?
	case "$blkid_status" in
	0 | 2) ;;
	*)
		echo >&2 "WARNING: blkid failed on $1 (exit status $blkid_status); not formatting it, as it may already hold data"
		return 1
		;;
	esac
	[ -z "$blkid_fstype" ]
}

for i in $(seq 0 $((LIMA_CIDATA_DISKS - 1))); do
	DISK_NAME="$(get_disk_var "$i" "NAME")"
	DEVICE_NAME="$(get_disk_var "$i" "DEVICE")"
	FORMAT_DISK="$(get_disk_var "$i" "FORMAT")"
	FORMAT_FSTYPE="$(get_disk_var "$i" "FSTYPE")"
	FORMAT_FSARGS="$(get_disk_var "$i" "FSARGS")"

	test -n "$FORMAT_DISK" || FORMAT_DISK=true
	test -n "$FORMAT_FSTYPE" || FORMAT_FSTYPE=ext4

	PARTITION="/dev/${DEVICE_NAME}1"

	# GPT partition names hold 36 UTF-16 code units. The label is only a
	# convenience for the user (/dev/disk/by-partlabel/), never the signal
	# tested below, so truncating an over-long name here is harmless.
	PARTLABEL="lima-${DISK_NAME}"
	PARTLABEL="${PARTLABEL:0:36}"

	# Clamp the filesystem label to what the filesystem accepts: mkfs.ext4
	# truncates an over-long label silently, but mkfs.xfs treats it as an error
	# and would abort the boot. Like PARTLABEL this is now only a convenience
	# for the user, so truncation costs nothing. For an fsType not listed here,
	# the limit (and even whether mkfs.$FORMAT_FSTYPE accepts a label at all)
	# is unknown, so no label is set rather than guess and risk aborting boot.
	case "$FORMAT_FSTYPE" in
	ext2 | ext3 | ext4) FSLABEL_MAX=16 ;;
	xfs) FSLABEL_MAX=12 ;;
	btrfs) FSLABEL_MAX=255 ;;
	swap) FSLABEL_MAX=15 ;;
	*) FSLABEL_MAX=0 ;;
	esac
	# FSLABEL is passed to mkfs as ${FSLABEL:+-L "$FSLABEL"}, not through an
	# array: on Alpine, /bin/bash is BusyBox ash (see
	# boot.essential.Linux/01-alpine-ash-as-bash.sh), which has no arrays.
	FSLABEL=""
	if [ "$FSLABEL_MAX" -gt 0 ]; then
		FSLABEL="lima-${DISK_NAME}"
		FSLABEL="${FSLABEL:0:FSLABEL_MAX}"
	fi

	# Do not identify the disk by its filesystem label. mkfs.ext4 silently
	# truncates the label to 16 bytes, so a DISK_NAME longer than 11 characters
	# is formatted with a label that can never match the "lima-${DISK_NAME}" the
	# guard looks for. The guard would then be true on every boot and mkfs would
	# run again, destroying the contents of the very disk the user created to
	# persist data.
	#
	# A partition that already carries a filesystem has been through first-time
	# setup, whatever label it ended up with. That also covers disks formatted
	# by older Lima versions, which must never be reformatted on upgrade.
	#
	# blkid is probed rather than tested through /dev/disk/by-*: those symlinks
	# depend on udev having probed the device, and can be absent or stale.
	if partition_needs_setup "$PARTITION"; then
		# first time setup
		if $FORMAT_DISK; then
			if [ "$FORMAT_FSTYPE" == "swap" ]; then
				echo "type=swap,name=${PARTLABEL}" | sfdisk --label gpt "/dev/${DEVICE_NAME}"
				# sfdisk's BLKRRPART emits a uevent for the new partition; settle
				# so the device node exists before mkswap opens it. Tolerate
				# failure: a missing udevadm must not kill boot under set -e.
				udevadm settle || true
				# shellcheck disable=SC2086
				mkswap $FORMAT_FSARGS ${FSLABEL:+-L "$FSLABEL"} "$PARTITION"
			else
				echo "type=linux,name=${PARTLABEL}" | sfdisk --label gpt "/dev/${DEVICE_NAME}"
				# As above: let the partition node appear before mkfs opens it.
				udevadm settle || true
				# shellcheck disable=SC2086
				mkfs.$FORMAT_FSTYPE $FORMAT_FSARGS ${FSLABEL:+-L "$FSLABEL"} "$PARTITION"
			fi
		fi
	fi

	if [ "$FORMAT_FSTYPE" == "swap" ]; then
		swapon "$PARTITION"
	else
		mkdir -p "/mnt/lima-${DISK_NAME}"
		mount -t "$FORMAT_FSTYPE" "$PARTITION" "/mnt/lima-${DISK_NAME}"
	fi
	if command -v growpart >/dev/null 2>&1 && command -v resize2fs >/dev/null 2>&1; then
		growpart "/dev/${DEVICE_NAME}" 1 || true
		# Address the partition directly rather than through
		# /dev/disk/by-label/: the label may have been truncated by mkfs, and
		# the symlink additionally depends on udev having probed the device.
		# Only resize when filesystem is in a healthy state
		if command -v "fsck.$FORMAT_FSTYPE" -f -p "$PARTITION"; then
			if [[ $FORMAT_FSTYPE == "ext2" || $FORMAT_FSTYPE == "ext3" || $FORMAT_FSTYPE == "ext4" ]]; then
				resize2fs "$PARTITION" || true
			elif [ "$FORMAT_FSTYPE" == "xfs" ]; then
				xfs_growfs "$PARTITION" || true
			else
				echo >&2 "WARNING: unknown fs '$FORMAT_FSTYPE'. FS will not be grew up automatically"
			fi
		fi
	fi
done
