#!/bin/sh
set -eux

# Check if cloud-init forgot to reboot_if_required
# (only implemented for apt at the moment, not dnf)

if command -v dnf >/dev/null 2>&1; then
	# dnf-utils needs to be installed, for needs-restarting
	if dnf -h needs-restarting >/dev/null 2>&1; then
		# needs-restarting returns "false" if needed (!)
		if ! dnf needs-restarting -r >/dev/null 2>&1; then
			systemctl reboot
		fi
	fi
fi
