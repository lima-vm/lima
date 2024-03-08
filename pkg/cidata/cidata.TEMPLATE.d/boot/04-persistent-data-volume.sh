#!/bin/bash
# bash is used for enabling `set -o pipefail`.
# NOTE: On Alpine, /bin/bash is ash with ASH_BASH_COMPAT, not GNU bash
set -eux -o pipefail

# Restrict the rest of this script to Alpine until it has been tested with other distros
test -f /etc/alpine-release || exit 0

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
		MNTPNT=${MNTPNT//\\/\\\\}
		MNTPNT=${MNTPNT//\"/\\\"}
		echo "mount -t \"${MNTTYPE}\" -o \"${MNTOPTS}\" \"${MNTDEV}\" \"${MNTPNT}\"" >>/mnt.sh
		# Before mv, unmount filesystems (virtiofs, 9p, etc.) below "${DIR}", otherwise host mounts will be wiped out
		# https://github.com/rancher-sandbox/rancher-desktop/issues/6582
		umount "${MNTPNT}" || exit 1
	done </proc/mounts
done
chmod +x /mnt.sh

# When running from RAM try to move persistent data to data-volume
# FIXME: the test for tmpfs mounts is probably Alpine-specific
if [ "$(awk '$2 == "/" {print $3}' /proc/mounts)" == "tmpfs" ]; then
	mkdir -p /mnt/data
	if [ -e /dev/disk/by-label/data-volume ]; then
		# Find which disk is data volume on
		DATA_DISK=$(blkid | grep "data-volume" | awk '{split($0,s,":"); sub(/\d$/, "", s[1]); print s[1]};')
		# growpart command may be missing in older VMs
		if command -v growpart >/dev/null 2>&1 && command -v resize2fs >/dev/null 2>&1; then
			# Automatically expand the data volume filesystem
			growpart "$DATA_DISK" 1 || true
			# Only resize when filesystem is in a healthy state
			if e2fsck -f -p /dev/disk/by-label/data-volume; then
				resize2fs /dev/disk/by-label/data-volume || true
			fi
		fi
		# Mount data volume
		mount -t ext4 /dev/disk/by-label/data-volume /mnt/data
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
			IN_USE=false
			# Looking for a disk that is not mounted or partitioned
			# shellcheck disable=SC2013
			for PART in $(awk '/^\/dev\// {gsub("/dev/", ""); print $1}' /proc/mounts); do
				if [ "${DISK}" == "${PART}" ] || [ -e /sys/block/"${DISK}"/"${PART}" ]; then
					IN_USE=true
					break
				fi
			done
			if [ "${IN_USE}" == "false" ]; then
				echo 'type=83' | sfdisk --label dos /dev/"${DISK}"
				PART=$(lsblk --list /dev/"${DISK}" --noheadings --output name,type | awk '$2 == "part" {print $1}')
				mkfs.ext4 -L data-volume /dev/"${PART}"
				mount -t ext4 /dev/disk/by-label/data-volume /mnt/data
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
			fi
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
fi
