#!/bin/bash

# SPDX-FileCopyrightText: Copyright The Lima Authors
# SPDX-License-Identifier: Apache-2.0

# This script converts the diffdisk of a Lima instance to ASIF format.
# It requires that the instance is stopped before conversion.
# Usage: hack/convert-diffdisk-to-asif.sh [instance-name]

set -eux -o pipefail

instance="${1:-asif-test}"

# Get instance dir
instance_dir=$(limactl list "${instance}" --format "{{.Dir}}") || {
	echo "Failed to get instance dir for ${instance}"
	exit 1
}

# Check diffdisk type
head4bytes="$(head -c 4 "${instance_dir}/diffdisk")"
case "${head4bytes}" in
shdw)
	echo "diffdisk is already in ASIF format"
	exit 1
	;;
QFI*)
	echo "diffdisk is in QCOW2 format"
	exit 1
	;;
*) ;;
esac

# Check instance state
instance_state="$(limactl list "${instance}" --format "{{.Status}}")" || {
	echo "Failed to get instance state for ${instance}"
	exit 1
}
[[ ${instance_state} == "Stopped" ]] || {
	echo "Instance ${instance} must be stopped"
	exit 1
}

# Create ASIF image
diskutil image create blank --fs none --format ASIF --size 100GiB "${instance_dir}/diffdisk.asif"

# Attach ASIF image (`hdiutil attach` does not support attaching ASIF)
attached_device=$(diskutil image attach -n "${instance_dir}/diffdisk.asif")

# Write `diffdisk` content to attached device using `dd` with `conv=sparse` option (`diskutil` does not support sparse)
dd if="${instance_dir}/diffdisk" of="${attached_device}" status=progress conv=sparse

# Detach the device (`diskutil unmountDisk` does not detach the device)
hdiutil detach "${attached_device}"

# Replace `diffdisk` with `diffdisk.asif`
mv "${instance_dir}/diffdisk" "${instance_dir}/diffdisk.raw"
mv "${instance_dir}/diffdisk.asif" "${instance_dir}/diffdisk"

echo "Converted diffdisk to ASIF format successfully"
