#!/bin/bash

# SPDX-FileCopyrightText: Copyright The Lima Authors
# SPDX-License-Identifier: Apache-2.0

set -eux

[ "$LIMA_CIDATA_UPGRADE_PACKAGES" = "1" ] || exit 0

# Check if cloud-init forgot to reboot_if_required
# (only implemented for apt at the moment, not dnf)

if command -v dnf >/dev/null 2>&1; then
	# dnf-utils needs to be installed, for needs-restarting
	if dnf -h needs-restarting >/dev/null 2>&1; then
		# check-update returns "false" (100) if updates (!)
		set +e
		dnf check-update >/dev/null
		if [ "$?" != "1" ]; then
			# needs-restarting messages are translated _()
			export LC_ALL=C.UTF-8
			logfile=$(mktemp)
			# needs-restarting returns "false" if needed (!)
			set -o pipefail
			dnf needs-restarting -r | tee "$logfile"
			if [ "$?" = "1" ]; then
				if grep -q "Reboot is required" "$logfile"; then
					systemctl reboot
				fi
			fi
			rm "$logfile"
		fi
	fi
fi
