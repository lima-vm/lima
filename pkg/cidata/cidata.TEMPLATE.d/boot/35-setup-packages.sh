#!/bin/sh
set -eux

update_fuse_conf() {
	# Modify /etc/fuse.conf (/etc/fuse3.conf) to allow "-o allow_root"
	if [ "${LIMA_CIDATA_MOUNTS}" -gt 0 ]; then
		fuse_conf="/etc/fuse.conf"
		if [ -e /etc/fuse3.conf ]; then
			fuse_conf="/etc/fuse3.conf"
		fi
		if ! grep -q "^user_allow_other" "${fuse_conf}"; then
			echo "user_allow_other" >>"${fuse_conf}"
		fi
	fi
}

# update_fuse_conf has to be called after installing all the packages,
# otherwise apt-get fails with conflict
if [ "${LIMA_CIDATA_MOUNTTYPE}" = "reverse-sshfs" ]; then
	update_fuse_conf
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
