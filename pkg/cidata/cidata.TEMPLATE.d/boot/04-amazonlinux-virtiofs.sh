#!/bin/bash

# SPDX-FileCopyrightText: Copyright The Lima Authors
# SPDX-License-Identifier: Apache-2.0

set -eux -o pipefail

# Workaround for Amazon Linux 2023
if [ -f /etc/os-release ] && grep -q "Amazon Linux" /etc/os-release; then
	# 1. Create missing mount.virtiofs helper
	if [ ! -e /sbin/mount.virtiofs ]; then
		cat >/sbin/mount.virtiofs <<'EOF'
#!/bin/sh
exec mount -i -t virtiofs "$@"
EOF
		chmod +x /sbin/mount.virtiofs
	fi

	# Cloud-init fails to mount 'mount0' because it doesn't recognize it as a device,
	# so we extract the intended path from user-data and mount it manually.
	USER_DATA="/var/lib/cloud/instance/user-data.txt"
	if [ -f "${USER_DATA}" ]; then
		MOUNT_POINT=$(grep "mount0" "${USER_DATA}" | awk -F, '{print $2}' | tr -d '[:space:]')
		if [ -n "${MOUNT_POINT}" ] && ! mountpoint -q "${MOUNT_POINT}"; then
			mkdir -p "${MOUNT_POINT}"
			mount -t virtiofs mount0 "${MOUNT_POINT}" || true
		fi
	fi
fi
