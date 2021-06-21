#!/bin/sh
set -eux

if [ "${LIMA_CIDATA_CONTAINERD_SYSTEM}" != 1 ] && [ "${LIMA_CIDATA_CONTAINERD_USER}" != 1 ]; then
	exit 0
fi

# This script does not work unless systemd is available
command -v systemctl >/dev/null 2>&1 || exit 0

if [ ! -x /usr/local/bin/nerdctl ]; then
	tar Cxzf /usr/local "${LIMA_CIDATA_MNT}"/nerdctl-full.tgz
fi

if [ "${LIMA_CIDATA_CONTAINERD_SYSTEM}" = 1 ]; then
	mkdir -p /etc/containerd
	cat >"/etc/containerd/config.toml" <<EOF
  version = 2
  [proxy_plugins]
    [proxy_plugins."stargz"]
      type = "snapshot"
      address = "/run/containerd-stargz-grpc/containerd-stargz-grpc.sock"
EOF
	systemctl enable --now containerd buildkit stargz-snapshotter
fi

if [ "${LIMA_CIDATA_CONTAINERD_USER}" = 1 ]; then
	modprobe tap || true
	if [ ! -e "/home/${LIMA_CIDATA_USER}.linux/.config/containerd/config.toml" ]; then
		mkdir -p "/home/${LIMA_CIDATA_USER}.linux/.config/containerd"
		cat >"/home/${LIMA_CIDATA_USER}.linux/.config/containerd/config.toml" <<EOF
  version = 2
  [proxy_plugins]
    [proxy_plugins."fuse-overlayfs"]
      type = "snapshot"
      address = "/run/user/${LIMA_CIDATA_UID}/containerd-fuse-overlayfs.sock"
    [proxy_plugins."stargz"]
      type = "snapshot"
      address = "/run/user/${LIMA_CIDATA_UID}/containerd-stargz-grpc/containerd-stargz-grpc.sock"
EOF
		chown -R "${LIMA_CIDATA_USER}" "/home/${LIMA_CIDATA_USER}.linux/.config"
	fi
	selinux=
	if command -v selinuxenabled >/dev/null 2>&1 && selinuxenabled; then
		selinux=1
	fi
	if [ ! -e "/home/${LIMA_CIDATA_USER}}}.linux/.config/systemd/user/containerd.service" ]; then
		until [ -e "/run/user/${LIMA_CIDATA_UID}/systemd/private" ]; do sleep 3; done
		if [ -n "$selinux" ]; then
			echo "Temporarily disabling SELinux, during installing containerd units"
			setenforce 0
		fi
		sudo -iu "${LIMA_CIDATA_USER}" "XDG_RUNTIME_DIR=/run/user/${LIMA_CIDATA_UID}" systemctl --user enable --now dbus
		sudo -iu "${LIMA_CIDATA_USER}" "XDG_RUNTIME_DIR=/run/user/${LIMA_CIDATA_UID}" containerd-rootless-setuptool.sh install
		sudo -iu "${LIMA_CIDATA_USER}" "XDG_RUNTIME_DIR=/run/user/${LIMA_CIDATA_UID}" containerd-rootless-setuptool.sh install-buildkit
		sudo -iu "${LIMA_CIDATA_USER}" "XDG_RUNTIME_DIR=/run/user/${LIMA_CIDATA_UID}" containerd-rootless-setuptool.sh install-fuse-overlayfs
		if ! sudo -iu "${LIMA_CIDATA_USER}" "XDG_RUNTIME_DIR=/run/user/${LIMA_CIDATA_UID}" containerd-rootless-setuptool.sh install-stargz; then
			echo >&2 "WARNING: rootless stargz does not seem supported on this host (kernel older than 5.11?)"
		fi
		if [ -n "$selinux" ]; then
			echo "Restoring SELinux"
			setenforce 1
		fi
	fi
fi
