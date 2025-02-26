#!/bin/bash

set -eux -o pipefail

if [ "$LIMA_CIDATA_ROSETTA_ENABLED" != "true" ]; then
	exit 0
fi

if [ -f /etc/alpine-release ]; then
	rc-service procfs start --ifnotstarted
	rc-service qemu-binfmt stop --ifexists --ifstarted
fi

binfmt_entry=/proc/sys/fs/binfmt_misc/rosetta
binfmtd_conf=/usr/lib/binfmt.d/rosetta.conf
if [ "$LIMA_CIDATA_ROSETTA_BINFMT" = "true" ]; then
	rosetta_binfmt=":rosetta:M::\x7fELF\x02\x01\x01\x00\x00\x00\x00\x00\x00\x00\x00\x00\x02\x00\x3e\x00:\xff\xff\xff\xff\xff\xfe\xfe\x00\xff\xff\xff\xff\xff\xff\xff\xff\xfe\xff\xff\xff:/mnt/lima-rosetta/rosetta:OCF"

	# If rosetta is not registered in binfmt_misc, register it.
	[ -f "$binfmt_entry" ] || echo "$rosetta_binfmt" >/proc/sys/fs/binfmt_misc/register

	# Create binfmt.d(5) configuration to prioritize rosetta even if qemu-user-static is installed on systemd based systems.
	# If the binfmt.d directory exists, consider systemd-binfmt.service(8) to be enabled and create the configuration file.
	[ ! -d "$(dirname "$binfmtd_conf")" ] || [ -f "$binfmtd_conf" ] || echo "$rosetta_binfmt" >"$binfmtd_conf"
else
	# unregister rosetta from binfmt_misc if it exists
	[ ! -f "$binfmt_entry" ] || echo -1 | "$binfmt_entry"
	# remove binfmt.d(5) configuration if it exists
	[ ! -f "$binfmtd_conf" ] || rm "$binfmtd_conf"
fi
