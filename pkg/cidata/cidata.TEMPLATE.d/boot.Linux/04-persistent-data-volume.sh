#!/bin/bash

# SPDX-FileCopyrightText: Copyright The Lima Authors
# SPDX-License-Identifier: Apache-2.0

# bash is used for enabling `set -o pipefail`.
# NOTE: On Alpine, /bin/bash is ash with ASH_BASH_COMPAT, not GNU bash
set -eux -o pipefail

# Restrict the rest of this script to Alpine until it has been tested with other distros
test -f /etc/alpine-release || exit 0

# Nothing to do unless we are running from a ramdisk
[ "$(awk '$2 == "/" {print $3}' /proc/mounts)" != "tmpfs" ] && exit 0

# Data directories that should be persisted across reboots
DATADIRS="/etc /home /root /tmp /usr/local /var/lib"

# Prepare mnt.sh (used for restoring mounts later)
echo "#!/bin/sh" >/mnt.sh
echo "set -eux" >>/mnt.sh
for DIR in ${DATADIRS}; do
	while IFS= read -r LINE; do
		[ -z "$LINE" ] && continue
		MNTDEV="$(echo "${LINE}" | awk '{print $1}')"
		# unmangle " \t\n\\#"
		# https://github.com/torvalds/linux/blob/v6.6/fs/proc_namespace.c#L89
		MNTPNT="$(echo "${LINE}" | awk '{print $2}' | sed -e 's/\\040/ /g; s/\\011/\t/g; s/\\012/\n/g; s/\\134/\\/g; s/\\043/#/g')"
		# Ignore if MNTPNT is neither DIR nor a parent directory of DIR.
		# It is not a parent if MNTPNT doesn't start with DIR, or the first
		# character after DIR isn't a slash.
		WITHOUT_DIR="${MNTPNT#"$DIR"}"
		# shellcheck disable=SC2166
		[ "$MNTPNT" != "$DIR" ] && [ "$MNTPNT" == "$WITHOUT_DIR" -o "${WITHOUT_DIR::1}" != "/" ] && continue
		MNTTYPE="$(echo "${LINE}" | awk '{print $3}')"
		[ "${MNTTYPE}" = "ext4" ] && continue
		[ "${MNTTYPE}" = "tmpfs" ] && continue
		MNTOPTS="$(echo "${LINE}" | awk '{print $4}')"
		if [ "${MNTTYPE}" = "9p" ]; then
			# https://github.com/torvalds/linux/blob/v6.6/fs/9p/v9fs.h#L61
			MNTOPTS="$(echo "${MNTOPTS}" | sed -e 's/cache=8f,/cache=fscache,/; s/cache=f,/cache=loose,/; s/cache=5,/cache=mmap,/; s/cache=1,/cache=readahead,/; s/cache=0,/cache=none,/')"
		fi
		# Before mv, unmount filesystems (virtiofs, 9p, etc.) below "${DIR}", otherwise host mounts will be wiped out
		# https://github.com/rancher-sandbox/rancher-desktop/issues/6582
		umount "${MNTPNT}" || exit 1
		MNTPNT=${MNTPNT//\\/\\\\}
		MNTPNT=${MNTPNT//\"/\\\"}
		echo "mount -t \"${MNTTYPE}\" -o \"${MNTOPTS}\" \"${MNTDEV}\" \"${MNTPNT}\"" >>/mnt.sh
	done </proc/mounts
done
chmod +x /mnt.sh

mkdir -p /mnt/data

# Resolve the data volume device via blkid, not the udev symlink.
# The /dev/disk/by-label/ symlink depends on udev having probed the device.
# A race between growpart (which triggers a partition table re-read and thus
# a udev re-probe) and e2fsck (which modifies the superblock) can cause the
# re-probe to see an inconsistent ext4 checksum and delete the symlink.
# BusyBox blkid doesn't support --label, so we parse the output instead.
DATA_VOLUME=$(blkid | sed -n 's/^\([^:]*\):.*LABEL="data-volume".*/\1/p')

if [ -n "${DATA_VOLUME}" ]; then
	DATA_DISK="${DATA_VOLUME%[0-9]}"
	# growpart command may be missing in older VMs
	if command -v growpart >/dev/null 2>&1 && command -v resize2fs >/dev/null 2>&1; then
		# Automatically expand the data volume filesystem
		growpart "$DATA_DISK" 1 || true
		# growpart triggers a partition table re-read; settle udev before
		# touching the device to avoid racing with the re-probe.
		udevadm settle
		# Only resize when filesystem is in a healthy state
		if e2fsck -f -p "${DATA_VOLUME}"; then
			resize2fs "${DATA_VOLUME}" || true
		fi
	fi
	# Mount data volume
	mount -t ext4 "${DATA_VOLUME}" /mnt/data
	# Update /etc files that might have changed during this boot
	cp /etc/network/interfaces /mnt/data/etc/network/
	cp /etc/resolv.conf /mnt/data/etc/
	if [ -f /etc/localtime ]; then
		# Preserve symlink
		cp -d /etc/localtime /mnt/data/etc/
		# setup-timezone copies the single zoneinfo file into /etc/zoneinfo and targets the symlink there
		if [ -d /etc/zoneinfo ]; then
			rm -rf /mnt/data/etc/zoneinfo
			cp -r /etc/zoneinfo /mnt/data/etc
		fi
	fi
	if [ -f /etc/timezone ]; then
		cp /etc/timezone /mnt/data/etc/
	fi
	# TODO there are probably others that should be updated as well
else
	# Find an unpartitioned disk and create data-volume
	DISKS=$(lsblk --list --noheadings --output name,type | awk '$2 == "disk" {print $1}')
	for DISK in ${DISKS}; do
		# A disk is in use if it has any partitions or is mounted directly.
		# Check lsblk for partitions, not just /proc/mounts; an unmounted
		# but partitioned disk (e.g. the data volume after a failed boot)
		# must not be reformatted.
		if lsblk --list --noheadings --output type /dev/"${DISK}" | grep --quiet "part"; then
			continue
		fi
		if awk '/^\/dev\// {gsub("/dev/", ""); print $1}' /proc/mounts | grep --quiet "^${DISK}$"; then
			continue
		fi
		echo 'type=83' | sfdisk --label dos /dev/"${DISK}"
		PART=$(lsblk --list /dev/"${DISK}" --noheadings --output name,type | awk '$2 == "part" {print $1}')
		mkfs.ext4 -L data-volume /dev/"${PART}"
		# Let udev process the new filesystem before continuing; mount
		# uses the device path directly, but later boot scripts or
		# services may depend on the /dev/disk/by-label/ symlink.
		udevadm settle
		mount -t ext4 /dev/"${PART}" /mnt/data
		# setup apk package cache
		mkdir -p /mnt/data/apk/cache
		mkdir -p /etc/apk
		ln -s /mnt/data/apk/cache /etc/apk/cache
		# Move all persisted directories to the data volume
		for DIR in ${DATADIRS}; do
			DEST="/mnt/data$(dirname "${DIR}")"
			mkdir -p "${DIR}" "${DEST}"
			mv "${DIR}" "${DEST}"
		done
		# Make sure all data moved to the persistent volume has been committed to disk
		sync
		break
	done
fi
for DIR in ${DATADIRS}; do
	if [ -d /mnt/data"${DIR}" ]; then
		mkdir -p "${DIR}"
		mount --bind /mnt/data"${DIR}" "${DIR}"
	fi
done
# Remount submounts on top of the new ${DIR}
/mnt.sh
# Reinstall packages from /mnt/data/apk/cache into the RAM disk
apk fix --no-network
