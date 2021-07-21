#!/bin/sh
set -eux

# Install containerd dependencies

if [ "${LIMA_CIDATA_CONTAINERD_SYSTEM}" != 1 ] && [ "${LIMA_CIDATA_CONTAINERD_USER}" != 1 ]; then
	exit 0
fi

if command -v apt-get >/dev/null 2>&1; then
	pkgs=
	if [ ! -e /usr/sbin/iptables ]; then
		pkgs="${pkgs} iptables"
	fi
	if [ "${LIMA_CIDATA_CONTAINERD_USER}" = 1 ]; then
		if [ ! -e /usr/lib/systemd/user/dbus.socket ]; then
			pkgs="${pkgs} dbus-user-session"
		fi
		if ! command -v newuidmap >/dev/null 2>&1; then
			pkgs="${pkgs} uidmap"
		fi
		if ! command -v mount.fuse3 >/dev/null 2>&1; then
			pkgs="${pkgs} fuse3"
		fi
	fi
	if [ -n "${pkgs}" ]; then
		DEBIAN_FRONTEND=noninteractive
		export DEBIAN_FRONTEND
		apt-get update
		# shellcheck disable=SC2086
		apt-get install -y -q ${pkgs}
	fi
elif command -v dnf >/dev/null 2>&1; then
	pkgs=
	if [ ! -e /usr/sbin/iptables ]; then
		pkgs="${pkgs} iptables"
	fi
	if [ "${LIMA_CIDATA_CONTAINERD_USER}" = 1 ]; then
		if [ ! -e /usr/lib/systemd/user/dbus.socket ]; then
			pkgs="${pkgs} dbus-daemon"
		fi
		if ! command -v newuidmap >/dev/null 2>&1; then
			pkgs="${pkgs} shadow-utils"
		fi
		if ! command -v mount.fuse3 >/dev/null 2>&1; then
			pkgs="${pkgs} fuse3"
		fi
	fi
	if [ -n "${pkgs}" ]; then
		# shellcheck disable=SC2086
		dnf install -y ${pkgs}
	fi
	if [ "${LIMA_CIDATA_CONTAINERD_USER}" = 1 ] && [ ! -e /usr/bin/fusermount ]; then
		# Workaround for https://github.com/containerd/stargz-snapshotter/issues/340
		ln -s fusermount3 /usr/bin/fusermount
	fi
elif command -v pacman >/dev/null 2>&1; then
	pkgs=
	# not a typo of "/usr/sbin/iptables"
	if [ ! -e /usr/bin/iptables ]; then
		pkgs="${pkgs} iptables"
	fi
	if [ "${LIMA_CIDATA_CONTAINERD_USER}" = 1 ]; then
		if [ ! -e /usr/lib/systemd/user/dbus.socket ]; then
			pkgs="${pkgs} dbus"
		fi
		if ! command -v newuidmap >/dev/null 2>&1; then
			pkgs="${pkgs} shadow"
		fi
		if ! command -v mount.fuse3 >/dev/null 2>&1; then
			pkgs="${pkgs} fuse3"
		fi
	fi
	if [ -n "${pkgs}" ]; then
		# shellcheck disable=SC2086
		pacman -Syu --noconfirm ${pkgs}
	fi
elif command -v zypper >/dev/null 2>&1; then
	pkgs=
	if [ ! -e /usr/sbin/iptables ]; then
		pkgs="${pkgs} iptables"
	fi
	if [ "${LIMA_CIDATA_CONTAINERD_USER}" = 1 ]; then
		if [ ! -e /usr/lib/systemd/user/dbus.socket ]; then
			pkgs="${pkgs} dbus-1"
		fi
		if ! command -v newuidmap >/dev/null 2>&1; then
			pkgs="${pkgs} shadow"
		fi
		if ! command -v mount.fuse3 >/dev/null 2>&1; then
			pkgs="${pkgs} fuse3"
		fi
	fi
	if [ -n "${pkgs}" ]; then
		# shellcheck disable=SC2086
		zypper install -y ${pkgs}
	fi
elif command -v apk >/dev/null 2>&1; then
	: NOP
	# our built-in containerd installer does not support Alpine yet
fi
