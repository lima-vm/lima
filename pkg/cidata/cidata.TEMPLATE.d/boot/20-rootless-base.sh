#!/bin/sh

# SPDX-FileCopyrightText: Copyright The Lima Authors
# SPDX-License-Identifier: Apache-2.0

set -eux

# This script does not work unless systemd is available
command -v systemctl >/dev/null 2>&1 || exit 0

if [ -O "${LIMA_CIDATA_HOME}" ]; then
	# Fix ownership of the user home directory when created by root.
	# In cases where mount points exist in the user's home directory, the home directory and
	# the mount points are created by root before the user is created. This leads to the home
	# directory being owned by root.
	# Following commands fix the ownership of the home directory and its contents (on the same filesystem)
	# is updated to the correct user.
	# shellcheck disable=SC2046 # it fails if find results are quoted.
	chown "${LIMA_CIDATA_USER}" $(find "${LIMA_CIDATA_HOME}" -xdev) ||
		true # Ignore errors because changing owner of the mount points may fail but it is not critical.
fi

# Set up env
for f in .profile .bashrc .zshrc; do
	if ! grep -q "# Lima BEGIN" "${LIMA_CIDATA_HOME}/$f"; then
		cat >>"${LIMA_CIDATA_HOME}/$f" <<EOF
# Lima BEGIN
# Make sure iptables and mount.fuse3 are available
PATH="\$PATH:/usr/sbin:/sbin"
export PATH
EOF
		if compare_version.sh "$(uname -r)" -lt "5.13"; then
			cat >>"${LIMA_CIDATA_HOME}/$f" <<EOF
# fuse-overlayfs is the most stable snapshotter for rootless, on kernel < 5.13
# https://github.com/lima-vm/lima/issues/383
# https://rootlesscontaine.rs/how-it-works/overlayfs/
CONTAINERD_SNAPSHOTTER="fuse-overlayfs"
export CONTAINERD_SNAPSHOTTER
EOF
		fi
		cat >>"${LIMA_CIDATA_HOME}/$f" <<EOF
# Lima END
EOF
		chown "${LIMA_CIDATA_USER}" "${LIMA_CIDATA_HOME}/$f"
	fi
done
# Enable cgroup delegation (only meaningful on cgroup v2)
if [ ! -e "/etc/systemd/system/user@.service.d/lima.conf" ]; then
	mkdir -p "/etc/systemd/system/user@.service.d"
	cat >"/etc/systemd/system/user@.service.d/lima.conf" <<EOF
[Service]
Delegate=yes
EOF
fi
systemctl daemon-reload

# Set up sysctl
sysctl_conf="/etc/sysctl.d/99-lima.conf"
if [ ! -e "${sysctl_conf}" ]; then
	if [ -e "/proc/sys/kernel/unprivileged_userns_clone" ]; then
		echo "kernel.unprivileged_userns_clone=1" >>"${sysctl_conf}"
	fi
	echo "net.ipv4.ping_group_range = 0 2147483647" >>"${sysctl_conf}"
	echo "net.ipv4.ip_unprivileged_port_start=0" >>"${sysctl_conf}"
	sysctl --system
fi

# Set up subuid
for f in /etc/subuid /etc/subgid; do
	# systemd-homed expects the subuid range to be within 524288-1878982656 (0x80000-0x6fff0000).
	# See userdbctl.
	# 1073741824 (1G) is just an arbitrary number.
	# 1073741825-1878982656 is left blank for additional accounts.
	grep -qw "${LIMA_CIDATA_USER}" $f || echo "${LIMA_CIDATA_USER}:524288:1073741824" >>$f
done

# Start systemd session
systemctl start systemd-logind.service
loginctl enable-linger "${LIMA_CIDATA_USER}"
