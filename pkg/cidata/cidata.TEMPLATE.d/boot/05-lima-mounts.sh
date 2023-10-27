#!/bin/bash

set -eux -o pipefail

# Check if mount type is virtiofs and vm type as vz
if ! [[ ${LIMA_CIDATA_VMTYPE} == "vz" && ${LIMA_CIDATA_MOUNTTYPE} == "virtiofs" ]]; then
	exit 0
fi

# Update fstab entries and unmount/remount the volumes with secontext options
# when selinux is enabled in kernel
if [ -d /sys/fs/selinux ]; then
	# shellcheck disable=SC2013
	for line in $(grep -n virtiofs </etc/fstab | cut -d':' -f1); do
		OPTIONS=$(awk -v line="$line" 'NR==line {print $4}' /etc/fstab)
		if [[ ${OPTIONS} != *"context"* ]]; then
			sed -i -e "$line""s/comment=cloudconfig/comment=cloudconfig,context=\"system_u:object_r:container_file_t:s0\"/g" /etc/fstab
			TAG=$(awk -v line="$line" 'NR==line {print $1}' /etc/fstab)
			MOUNT_POINT=$(awk -v line="$line" 'NR==line {print $2}' /etc/fstab)
			OPTIONS=$(awk -v line="$line" 'NR==line {print $4}' /etc/fstab)
			umount "${TAG}"
			mount -t virtiofs "${TAG}" "${MOUNT_POINT}" -o "${OPTIONS}"
		fi
	done
fi
