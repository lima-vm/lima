#!/bin/sh

# SPDX-FileCopyrightText: Copyright The Lima Authors
# SPDX-License-Identifier: Apache-2.0

# Populate mounts from the cidata.
# This script is a workaround until nuageinit supports mounts.

set -eux

if ! grep -q '#LIMA-START' /etc/fstab; then
	"${LIMA_CIDATA_MNT}"/util.FreeBSD/print_cidata_fstab.lua >/etc/fstab.lima.tmp
	if [ -s /etc/fstab.lima.tmp ]; then
		{
			echo "#LIMA-START"
			cat /etc/fstab.lima.tmp
			echo "#LIMA-END"
		} >>/etc/fstab
	fi
	rm -f /etc/fstab.lima.tmp
fi

# Run mkdir on every boot, as the mount points may be on tmpfs
awk '/^[^#]/ && $2 != "none" {print $2}' /etc/fstab | while IFS= read -r line; do
	mkdir -p "${line}"
done
mount -a
