#!/bin/bash

set -eux -o pipefail

test "$LIMA_CIDATA_DISKS" -gt 0 || exit 0

get_disk_var() {
	diskvarname="LIMA_CIDATA_DISK_${1}_${2}"
	eval echo \$"$diskvarname"
}

for i in $(seq 0 $((LIMA_CIDATA_DISKS - 1))); do
	DISK_NAME="$(get_disk_var "$i" "NAME")"
	DEVICE_NAME="$(get_disk_var "$i" "DEVICE")"

	# first time setup
	if [[ ! -b "/dev/disk/by-label/lima-${DISK_NAME}" ]]; then
		# TODO: skip if disk is tagged as "raw"
		echo 'type=linux' | sfdisk --label gpt "/dev/${DEVICE_NAME}"
		mkfs.ext4 -L "lima-${DISK_NAME}" "/dev/${DEVICE_NAME}1"
	fi

	mkdir -p "/mnt/lima-${DISK_NAME}"
	mount -t ext4 "/dev/${DEVICE_NAME}1" "/mnt/lima-${DISK_NAME}"
done
