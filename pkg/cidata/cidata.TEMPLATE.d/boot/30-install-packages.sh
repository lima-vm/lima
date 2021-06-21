#!/bin/sh
set -eux

# Install minimum dependencies
if command -v apt-get 2>&1 >/dev/null; then
  DEBIAN_FRONTEND=noninteractive
  export DEBIAN_FRONTEND
  apt-get update
  if [ "${LIMA_CIDATA_MOUNTS}" -gt 0 ]; then
    apt-get install -y sshfs
  fi
  if [ "${LIMA_CIDATA_CONTAINERD_SYSTEM}" = 1 ] || [ "${LIMA_CIDATA_CONTAINERD_USER}" = 1 ]; then
    apt-get install -y iptables
  fi
  if [ "${LIMA_CIDATA_CONTAINERD_USER}" = 1 ]; then
    apt-get install -y uidmap fuse3 dbus-user-session
  fi
elif command -v dnf 2>&1 >/dev/null; then
  if [ "${LIMA_CIDATA_MOUNTS}" -gt 0 ]; then
    dnf install -y fuse-sshfs
  fi
  if [ "${LIMA_CIDATA_CONTAINERD_SYSTEM}" = 1 ] || [ "${LIMA_CIDATA_CONTAINERD_USER}" = 1 ]; then
    dnf install -y iptables
  fi
  if [ "${LIMA_CIDATA_CONTAINERD_USER}" = 1 ]; then
    dnf install -y shadow-utils fuse3
    if [ ! -f /usr/bin/fusermount ]; then
      # Workaround for https://github.com/containerd/stargz-snapshotter/issues/340
      ln -s fusermount3 /usr/bin/fusermount
    fi
  fi
elif command -v apk 2>&1 >/dev/null; then
  if [ "${LIMA_CIDATA_MOUNTS}" -gt 0 ]; then
    if ! command -v sshfs 2>&1 >/dev/null; then
      apk update
      apk add sshfs
    fi
    modprobe fuse
  fi
fi
# Modify /etc/fuse.conf to allow "-o allow_root"

if [ "${LIMA_CIDATA_MOUNTS}" -gt 0 ]; then
  if ! grep -q "^user_allow_other" /etc/fuse.conf ; then
    echo "user_allow_other" >> /etc/fuse.conf
  fi
fi
