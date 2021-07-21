#!/bin/sh
set -eux

# Install mount.cifs dependencies
if command -v apt-get >/dev/null 2>&1; then
	DEBIAN_FRONTEND=noninteractive
	export DEBIAN_FRONTEND
	apt-get update
	if [ "${LIMA_CIDATA_MOUNTS}" -gt 0 ]; then
		if ! command -v mount.cifs >/dev/null 2>&1; then
			apt-get install -y cifs-utils
		fi
	fi
fi
