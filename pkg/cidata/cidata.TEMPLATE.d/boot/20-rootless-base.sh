#!/bin/sh
set -eux

# This script does not work unless systemd is available
command -v systemctl >/dev/null 2>&1 || exit 0

# Set up env
for f in .profile .bashrc; do
	if ! grep -q "# Lima BEGIN" "/home/${LIMA_CIDATA_USER}.linux/$f"; then
		cat >>"/home/${LIMA_CIDATA_USER}.linux/$f" <<EOF
# Lima BEGIN
# Make sure iptables and mount.fuse3 are available
PATH="$PATH:/usr/sbin:/sbin"
# fuse-overlayfs is the most stable snapshotter for rootless
CONTAINERD_SNAPSHOTTER="fuse-overlayfs"
export PATH CONTAINERD_SNAPSHOTTER
# Lima END
EOF
		chown "${LIMA_CIDATA_USER}" "/home/${LIMA_CIDATA_USER}.linux/$f"
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
	grep -qw "${LIMA_CIDATA_USER}" $f || echo "${LIMA_CIDATA_USER}:100000:65536" >>$f
done

# Start systemd session
loginctl enable-linger "${LIMA_CIDATA_USER}"
