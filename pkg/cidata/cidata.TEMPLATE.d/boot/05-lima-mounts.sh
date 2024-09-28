#!/bin/bash

set -eux -o pipefail

# Check if mount type is virtiofs and vm type as vz
if ! [[ ${LIMA_CIDATA_VMTYPE} == "vz" && ${LIMA_CIDATA_MOUNTTYPE} == "virtiofs" ]]; then
	exit 0
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
				MOUNT_POINT=$(awk -v line="$line" 'NR==line {print $2}' /etc/fstab)
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
