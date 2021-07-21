#!/bin/sh
set -ux

code=0

grep -q -w "cifs" /proc/filesystems || modprobe cifs

# openSUSE 15.3 doesn't seem to provide nls_utf8.ko, though /proc/config.gz contains CONFIG_NLS_UTF8=m
# https://bugzilla.opensuse.org/show_bug.cgi?id=1190797
nls_utf8_avail=
if grep -q -w "nls_utf8" /proc/modules || modprobe nls_utf8 || zgrep -q "CONFIG_NLS_UTF8=y" /proc/config.gz; then
	nls_utf8_avail=1
else
	echo >&2 "nls_utf8 module is not loaded, may cause issues with non-ASCII file names"
fi

# enable basic debug messages (https://www.kernel.org/doc/readme/Documentation-filesystems-cifs-README)
echo 3 >/proc/fs/cifs/cifsFYI

if [ ! -e "/sys/firmware/qemu_fw_cfg" ]; then
	modprobe qemu_fw_cfg
fi
credentials="/sys/firmware/qemu_fw_cfg/by_name/opt/io.github.lima-vm.lima.samba-credentials/raw"

set -e
if [ ! -e "${credentials}" ]; then
	echo >&2 "not found: ${credentials}"
	exit 1
fi

# NOTE: Busybox sh does not support `for ((i=0;i<$N;i++))` form
for f in $(seq 0 $((LIMA_CIDATA_MOUNTS - 1))); do
	mountpointvar="LIMA_CIDATA_MOUNTS_${f}_MOUNTPOINT"
	mountpoint="$(eval echo \$"$mountpointvar")"
	writablevar="LIMA_CIDATA_MOUNTS_${f}_WRITABLE"
	writable="$(eval echo \$"$writablevar")"

	mkdir -p "${mountpoint}"

	src="//${LIMA_CIDATA_SLIRP_FILESERVER}/lima-${f}"

	# Enable SMB1, to support unix extensions
	# FIXME: use SMB3.1.1 when Samba supports unix extensions for SMB3.1.1
	# https://bugs.launchpad.net/ubuntu/+source/samba/+bug/1883234
	o="vers=1.0"
	if [ "${writable}" = 1 ]; then
		o="$o,rw"
	else
		o="$o,ro"
	fi
	if [ "${nls_utf8_avail}" = 1 ]; then
		o="$o,iocharset=utf8"
	fi

	o="$o,credentials=${credentials}"

	# TODO: consider writing these configs to /etc/fstab
	if ! mount -vvv -t cifs -o "${o}" "${src}" "${mountpoint}"; then
		code=1
		echo "Failed to mount ${mountpoint}"
	fi
done

# Show mounts, for debugging
cat /proc/mounts

if [ "${code}" != "0" ]; then
	dmesg
fi
exit "${code}"
