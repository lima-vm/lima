#!/bin/sh
set -eux

# Install mount.cifs dependencies
if [ "${LIMA_CIDATA_MOUNTS}" = 0 ] || command -v mount.cifs >/dev/null 2>&1; then
	exit 0
fi

if command -v apt-get >/dev/null 2>&1; then
	pkgs="cifs-utils"
	if uname -a | grep -q -i "Ubuntu"; then
		# install nls_utf8.ko for mounting cifs with iocharset=utf8
		# This module is installed by default on Debian, but not by default on Ubuntu.
		pkgs="${pkgs} linux-modules-extra-$(uname -r)"
	fi
	DEBIAN_FRONTEND=noninteractive
	export DEBIAN_FRONTEND
	apt-get update
	# shellcheck disable=SC2086
	apt-get install -y -q $pkgs
elif command -v dnf >/dev/null 2>&1; then
	dnf install -y cifs-utils
elif command -v pacman >/dev/null 2>&1; then
	pacman -Syu --noconfirm cifs-utils
elif command -v zypper >/dev/null 2>&1; then
	zypper install -y cifs-utils
	# openSUSE 15.3 doesn't seem to provide nls_utf8.ko, though /proc/config.gz contains CONFIG_NLS_UTF8=m
	# https://bugzilla.opensuse.org/show_bug.cgi?id=1190797
elif command -v apk >/dev/null 2>&1; then
	apk -v add cifs-utils
fi
