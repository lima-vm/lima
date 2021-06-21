#!/bin/bash
set -eux -o pipefail

# This script does not work unless systemd is available
command -v systemctl 2>&1 >/dev/null || exit 0

# Set up env
for f in .profile .bashrc; do
  if ! grep -q "# Lima BEGIN" "/home/{{.User}}.linux/$f"; then
    cat >>"/home/{{.User}}.linux/$f" <<EOF
# Lima BEGIN
# Make sure iptables and mount.fuse3 are available
PATH="$PATH:/usr/sbin:/sbin"
# fuse-overlayfs is the most stable snapshotter for rootless
CONTAINERD_SNAPSHOTTER="fuse-overlayfs"
export PATH CONTAINERD_SNAPSHOTTER
# Lima END
EOF
    chown "{{.User}}" "/home/{{.User}}.linux/$f"
  fi
done
# Enable cgroup delegation (only meaningful on cgroup v2)
if [ ! -e "/etc/systemd/system/user@.service.d/lima.conf" ]; then
  mkdir -p "/etc/systemd/system/user@.service.d"
  cat >"/etc/systemd/system/user@.service.d/lima.conf"  <<EOF
[Service]
Delegate=yes
EOF
fi
systemctl daemon-reload

# Set up sysctl
sysctl_conf="/etc/sysctl.d/99-lima.conf"
if [ ! -e "${sysctl_conf}" ]; then
  if [ -e "/proc/sys/kernel/unprivileged_userns_clone" ]; then
    echo "kernel.unprivileged_userns_clone=1" >> "${sysctl_conf}"
  fi
  echo "net.ipv4.ping_group_range = 0 2147483647" >> "${sysctl_conf}"
  echo "net.ipv4.ip_unprivileged_port_start=0" >> "${sysctl_conf}"
  sysctl --system
fi

# Set up subuid
for f in /etc/subuid /etc/subgid; do
  grep -qw "{{.User}}" $f || echo "{{.User}}:100000:65536" >> $f
done

# Start systemd session
loginctl enable-linger "{{.User}}"
