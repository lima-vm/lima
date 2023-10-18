#!/bin/bash

set -eux -o pipefail

if [ "$LIMA_CIDATA_ROSETTA_ENABLED" != "true" ]; then
	exit 0
fi

if [ -f /etc/alpine-release ]; then
	rc-service qemu-binfmt stop --ifstarted
fi

mkdir -p /mnt/lima-rosetta

#Check selinux is enabled by kernel
if [ -d /sys/fs/selinux ]; then
	mount -t virtiofs vz-rosetta /mnt/lima-rosetta -o context="system_u:object_r:container_file_t:s0"
else
	mount -t virtiofs vz-rosetta /mnt/lima-rosetta
fi

if [ "$LIMA_CIDATA_ROSETTA_BINFMT" = "true" ]; then
	echo \
		':rosetta:M::\x7fELF\x02\x01\x01\x00\x00\x00\x00\x00\x00\x00\x00\x00\x02\x00\x3e\x00:\xff\xff\xff\xff\xff\xfe\xfe\x00\xff\xff\xff\xff\xff\xff\xff\xff\xfe\xff\xff\xff:/mnt/lima-rosetta/rosetta:OCF' \
		>/proc/sys/fs/binfmt_misc/register
fi
