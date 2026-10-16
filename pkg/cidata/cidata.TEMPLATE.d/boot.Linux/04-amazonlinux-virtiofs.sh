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

	# Amazon Linux 2023 requires a workaround for virtiofs mounts.
	# The mount.virtiofs helper script is missing, and cloud-init
	# may fail to mount the filesystems. This creates the helper
	# and manually mounts them.
	USER_DATA="/var/lib/cloud/instance/user-data.txt"
	if [ -f "${USER_DATA}" ]; then
		# Parse all mount entries from user-data (mount0, mount1, ...)
		MOUNT_ENTRIES=$(grep -E '^\s*-\s+\[mount[0-9]+,' "${USER_DATA}" || true)
		if [ -n "${MOUNT_ENTRIES}" ]; then
			echo "${MOUNT_ENTRIES}" | while IFS= read -r entry; do
				MOUNT_TAG=$(echo "${entry}" | grep -oP 'mount\K[0-9]+')
				MOUNT_POINT=$(echo "${entry}" | awk -F, '{print $2}' | tr -d '[:space:]')
				if [ -n "${MOUNT_TAG}" ] && [ -n "${MOUNT_POINT}" ] && ! mountpoint -q "${MOUNT_POINT}"; then
					mkdir -p "${MOUNT_POINT}"
					mount -t virtiofs "mount${MOUNT_TAG}" "${MOUNT_POINT}" || true
				fi
			done
		fi
	fi
fi
