#!/bin/sh

# SPDX-FileCopyrightText: Copyright The Lima Authors
# SPDX-License-Identifier: Apache-2.0

# Load modules as soon as the cloud-init starts up.
# Because Arch Linux removes kernel module files when the kernel package was updated during running cloud-init :(

set -eu
for f in \
	fuse \
	tun tap \
	bridge veth \
	ip_tables ip6_tables iptable_nat ip6table_nat iptable_filter ip6table_filter \
	nf_tables \
	x_tables xt_MASQUERADE xt_addrtype xt_comment xt_conntrack xt_mark xt_multiport xt_nat xt_tcpudp \
	overlay; do
	echo "Loading kernel module \"$f\""
	if ! modprobe "$f"; then
		echo >&2 "Failed to load \"$f\" (negligible if it is built-in the kernel)"
	fi
done
