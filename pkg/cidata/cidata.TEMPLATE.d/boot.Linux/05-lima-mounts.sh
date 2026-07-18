#!/bin/bash

# SPDX-FileCopyrightText: Copyright The Lima Authors
# SPDX-License-Identifier: Apache-2.0

set -eux -o pipefail

# Check if mount type is virtiofs and vm type as vz
if ! [[ ${LIMA_CIDATA_VMTYPE} == "vz" && ${LIMA_CIDATA_MOUNTTYPE} == "virtiofs" ]]; then
	exit 0
fi

# cloud-init's cc_mounts writes the mount point into /etc/fstab without octal-escaping it, so a
# space/tab in the path makes an unparsable line that mount(8) silently skips via nofail.
# cc_mounts already created the directory from the unescaped value, so just repair the fstab
# syntax (see util/escape_fstab.sh) and (re)mount.
# https://github.com/lima-vm/lima/issues/5136
# https://github.com/abiosoft/colima/issues/1471
if grep -q virtiofs /etc/fstab; then
	escape_fstab.sh </etc/fstab >/etc/fstab.lima.tmp &&
		cp /etc/fstab.lima.tmp /etc/fstab && rm -f /etc/fstab.lima.tmp
	# Mount entries cc_mounts skipped due to the previously broken line (already-mounted ones
	# are a no-op). On Oracle Linux the virtiofs module is installed later in
	# 30-install-packages.sh (REMOUNT_VIRTIOFS=1 remounts then), so tolerate failure here.
	mount -t virtiofs -a || true
fi

# Update fstab entries and unmount/remount the volumes with secontext options
# when selinux is enabled in kernel
if [ -d /sys/fs/selinux ]; then
	LABEL_BIN="system_u:object_r:bin_t:s0"
	LABEL_NFS="system_u:object_r:nfs_t:s0"
	# shellcheck disable=SC2013
	for line in $(grep -n virtiofs </etc/fstab | cut -d':' -f1); do
		OPTIONS=$(awk -v line="$line" 'NR==line {print $4}' /etc/fstab)
		TAG=$(awk -v line="$line" 'NR==line {print $1}' /etc/fstab)
		MOUNT_OPTIONS=$(mount | grep "${TAG}" | awk '{print $6}')
		if [[ ${OPTIONS} != *"context"* ]]; then
			##########################################################################################
			## When using vz & virtiofs, initially container_file_t selinux label
			## was considered which works perfectly for container work loads
			## but it might break for other work loads if the process is running with
			## different label. Also these are the remote mounts from the host machine,
			## so keeping the label as nfs_t fits right. Package container-selinux by
			## default adds rules for nfs_t context which allows container workloads to work as well.
			## https://github.com/lima-vm/lima/pull/1965
			##
			## With integration[https://github.com/lima-vm/lima/pull/2474] with systemd-binfmt,
			## the existing "nfs_t" selinux label for Rosetta is causing issues while registering it.
			## This behaviour needs to be fixed by setting the label as "bin_t"
			## https://github.com/lima-vm/lima/pull/2630
			##########################################################################################
			if [[ ${TAG} == *"rosetta"* ]]; then
				label=${LABEL_BIN}
			else
				label=${LABEL_NFS}
			fi
			sed -i -e "$line""s/comment=cloudconfig/comment=cloudconfig,context=\"$label\"/g" /etc/fstab
			if [[ ${MOUNT_OPTIONS} != *"$label"* ]]; then
				# fstab stores the mount point octal-escaped (e.g. space = "\040"); decode
				# it before passing the path to mount(8).
				MOUNT_POINT=$(awk -v line="$line" 'NR==line {print $2}' /etc/fstab | unescape_fstab.sh)
				OPTIONS=$(awk -v line="$line" 'NR==line {print $4}' /etc/fstab)

				#########################################################
				## We need to migrate existing users of Fedora having
				## Rosetta mounted from nfs_t to bin_t by unregistering
				## it from binfmt before remounting
				#########################################################
				if [[ ${TAG} == *"rosetta"* && ${MOUNT_OPTIONS} == *"${LABEL_NFS}"* ]]; then
					[ ! -f "/proc/sys/fs/binfmt_misc/rosetta" ] || echo -1 >/proc/sys/fs/binfmt_misc/rosetta
				fi
				umount "${TAG}"
				mount -t virtiofs "${TAG}" "${MOUNT_POINT}" -o "${OPTIONS}"
			fi
		fi
	done
fi
