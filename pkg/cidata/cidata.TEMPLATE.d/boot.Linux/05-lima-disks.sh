#!/bin/bash

# SPDX-FileCopyrightText: Copyright The Lima Authors
# SPDX-License-Identifier: Apache-2.0

set -eux -o pipefail

test "$LIMA_CIDATA_DISKS" -gt 0 || exit 0

get_disk_var() {
	diskvarname="LIMA_CIDATA_DISK_${1}_${2}"
	eval echo \$"$diskvarname"
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
	FSLABEL=""
	if [ "$FSLABEL_MAX" -gt 0 ]; then
		FSLABEL="lima-${DISK_NAME}"
		FSLABEL="${FSLABEL:0:FSLABEL_MAX}"
	fi
	LABEL_ARGS=()
	if [ -n "$FSLABEL" ]; then
		LABEL_ARGS=(-L "$FSLABEL")
	fi

	# Do not identify the disk by its filesystem label. mkfs silently truncates
	# labels to a filesystem-dependent length (16 bytes on ext4, 12 on xfs), so
	# a sufficiently long DISK_NAME is formatted with a label that can never
	# match the "lima-${DISK_NAME}" the guard looks for. The guard would then be
	# true on every boot and mkfs would run again, destroying the contents of
	# the very disk the user created to persist data.
	#
	# A partition that already carries a filesystem has been through first-time
	# setup, whatever label it ended up with. That also covers disks formatted
	# by older Lima versions, which must never be reformatted on upgrade.
	#
	# blkid is probed rather than tested through /dev/disk/by-*: those symlinks
	# depend on udev having probed the device, and can be absent or stale.
	#
	# The TYPE tag is queried specifically, not just whether blkid recognizes
	# the partition at all: on GPT, blkid reports PARTUUID/PARTLABEL for a bare
	# partition-table entry before it ever has a filesystem, so testing for any
	# blkid output would treat a boot interrupted between sfdisk and mkfs as
	# already formatted -- and then fail at mount/swapon on every later boot.
	if ! blkid -o value -s TYPE "$PARTITION" >/dev/null 2>&1; then
		# first time setup
		if $FORMAT_DISK; then
			if [ "$FORMAT_FSTYPE" == "swap" ]; then
				echo "type=swap,name=${PARTLABEL}" | sfdisk --label gpt "/dev/${DEVICE_NAME}"
				# sfdisk's BLKRRPART emits a uevent for the new partition; settle
				# so the device node exists before mkswap opens it. Tolerate
				# failure: a missing udevadm must not kill boot under set -e.
				udevadm settle || true
				# shellcheck disable=SC2086
				mkswap $FORMAT_FSARGS "${LABEL_ARGS[@]}" "$PARTITION"
			else
				echo "type=linux,name=${PARTLABEL}" | sfdisk --label gpt "/dev/${DEVICE_NAME}"
				# As above: let the partition node appear before mkfs opens it.
				udevadm settle || true
				# shellcheck disable=SC2086
				mkfs.$FORMAT_FSTYPE $FORMAT_FSARGS "${LABEL_ARGS[@]}" "$PARTITION"
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
