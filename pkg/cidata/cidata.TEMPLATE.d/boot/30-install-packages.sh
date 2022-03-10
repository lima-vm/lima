#!/bin/sh
set -eux

update_fuse_conf() {
	# Modify /etc/fuse.conf to allow "-o allow_root"
	if [ "${LIMA_CIDATA_MOUNTS}" -gt 0 ]; then
		if ! grep -q "^user_allow_other" /etc/fuse.conf; then
			echo "user_allow_other" >>/etc/fuse.conf
		fi
	fi
}

INSTALL_IPTABLES=0
if [ "${LIMA_CIDATA_CONTAINERD_SYSTEM}" = 1 ] || [ "${LIMA_CIDATA_CONTAINERD_USER}" = 1 ]; then
	INSTALL_IPTABLES=1
fi
if [ "${LIMA_CIDATA_UDP_DNS_LOCAL_PORT}" -ne 0 ] || [ "${LIMA_CIDATA_TCP_DNS_LOCAL_PORT}" -ne 0 ]; then
	INSTALL_IPTABLES=1
fi

# Install minimum dependencies
if command -v apt-get >/dev/null 2>&1; then
	pkgs=""
	if [ "${LIMA_CIDATA_MOUNTS}" -gt 0 ] && ! command -v sshfs >/dev/null 2>&1; then
		pkgs="${pkgs} sshfs"
	fi
	if [ "${INSTALL_IPTABLES}" = 1 ] && [ ! -e /usr/sbin/iptables ]; then
		pkgs="${pkgs} iptables"
	fi
	if [ "${LIMA_CIDATA_CONTAINERD_USER}" = 1 ] && ! command -v newuidmap >/dev/null 2>&1; then
		pkgs="${pkgs} uidmap fuse3 dbus-user-session"
	fi
	if [ -n "${pkgs}" ]; then
		DEBIAN_FRONTEND=noninteractive
		export DEBIAN_FRONTEND
		apt-get update
		# shellcheck disable=SC2086
		apt-get install -y --no-upgrade --no-install-recommends -q ${pkgs}
	fi
elif command -v dnf >/dev/null 2>&1; then
	pkgs=""
	if ! command -v tar >/dev/null 2>&1; then
		pkgs="${pkgs} tar"
	fi
	if [ "${LIMA_CIDATA_MOUNTS}" -gt 0 ] && ! command -v sshfs >/dev/null 2>&1; then
		pkgs="${pkgs} fuse-sshfs"
	fi
	if [ "${INSTALL_IPTABLES}" = 1 ] && [ ! -e /usr/sbin/iptables ]; then
		pkgs="${pkgs} iptables"
	fi
	if [ "${LIMA_CIDATA_CONTAINERD_USER}" = 1 ]; then
		if ! command -v newuidmap >/dev/null 2>&1; then
			pkgs="${pkgs} shadow-utils"
		fi
		if ! command -v mount.fuse3 >/dev/null 2>&1; then
			pkgs="${pkgs} fuse3"
		fi
	fi
	if [ -n "${pkgs}" ]; then
		dnf_install_flags="-y --setopt=install_weak_deps=False"
		if grep -q "Oracle Linux Server release 8" /etc/system-release; then
			# repo flag instead of enable repo to reduce metadata syncing on slow Oracle repos
			dnf_install_flags="${dnf_install_flags} --repo ol8_baseos_latest --repo ol8_codeready_builder"
		elif grep -q "release 8" /etc/system-release; then
			dnf_install_flags="${dnf_install_flags} --enablerepo powertools"
		fi
		# shellcheck disable=SC2086
		dnf install ${dnf_install_flags} ${pkgs}
	fi
	if [ "${LIMA_CIDATA_CONTAINERD_USER}" = 1 ] && [ ! -e /usr/bin/fusermount ]; then
		# Workaround for https://github.com/containerd/stargz-snapshotter/issues/340
		ln -s fusermount3 /usr/bin/fusermount
	fi
elif command -v pacman >/dev/null 2>&1; then
	pkgs=""
	if [ "${LIMA_CIDATA_MOUNTS}" -gt 0 ] && ! command -v sshfs >/dev/null 2>&1; then
		pkgs="${pkgs} sshfs"
	fi
	# other dependencies are preinstalled on Arch Linux
	if [ -n "${pkgs}" ]; then
		# shellcheck disable=SC2086
		pacman -Sy --noconfirm ${pkgs}
	fi
elif command -v zypper >/dev/null 2>&1; then
	pkgs=""
	if [ "${LIMA_CIDATA_MOUNTS}" -gt 0 ] && ! command -v sshfs >/dev/null 2>&1; then
		pkgs="${pkgs} sshfs"
	fi
	if [ "${INSTALL_IPTABLES}" = 1 ] && [ ! -e /usr/sbin/iptables ]; then
		pkgs="${pkgs} iptables"
	fi
	if [ "${LIMA_CIDATA_CONTAINERD_USER}" = 1 ] && ! command -v mount.fuse3 >/dev/null 2>&1; then
		pkgs="${pkgs} fuse3"
	fi
	if [ -n "${pkgs}" ]; then
		# shellcheck disable=SC2086
		zypper --non-interactive install -y --no-recommends ${pkgs}
	fi
elif command -v apk >/dev/null 2>&1; then
	pkgs=""
	if [ "${LIMA_CIDATA_MOUNTS}" -gt 0 ] && ! command -v sshfs >/dev/null 2>&1; then
		pkgs="${pkgs} sshfs"
	fi
	if [ "${INSTALL_IPTABLES}" = 1 ] && ! command -v iptables >/dev/null 2>&1; then
		pkgs="${pkgs} iptables"
	fi
	if [ -n "${pkgs}" ]; then
		apk update
		# shellcheck disable=SC2086
		apk add ${pkgs}
	fi
fi

SETUP_DNS=0
if [ -n "${LIMA_CIDATA_UDP_DNS_LOCAL_PORT}" ] && [ "${LIMA_CIDATA_UDP_DNS_LOCAL_PORT}" -ne 0 ]; then
	SETUP_DNS=1
fi
if [ -n "${LIMA_CIDATA_TCP_DNS_LOCAL_PORT}" ] && [ "${LIMA_CIDATA_TCP_DNS_LOCAL_PORT}" -ne 0 ]; then
	SETUP_DNS=1
fi
if [ "${SETUP_DNS}" = 1 ]; then
	# Try to setup iptables rule again, in case we just installed iptables
	"${LIMA_CIDATA_MNT}/boot/09-host-dns-setup.sh"
fi

# update_fuse_conf has to be called after installing all the packages,
# otherwise apt-get fails with conflict
update_fuse_conf
