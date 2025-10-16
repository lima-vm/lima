#!/bin/bash

# SPDX-FileCopyrightText: Copyright The Lima Authors
# SPDX-License-Identifier: Apache-2.0

set -eux

if [ "${LIMA_CIDATA_MOUNTTYPE}" = "reverse-sshfs" ]; then
	# Create mount points
	# NOTE: Busybox sh does not support `for ((i=0;i<$N;i++))` form
	for f in $(seq 0 $((LIMA_CIDATA_MOUNTS - 1))); do
		mountpointvar="LIMA_CIDATA_MOUNTS_${f}_MOUNTPOINT"
		mountpoint="$(eval echo \$"$mountpointvar")"
		mkdir -p "${mountpoint}"
		gid=$(id -g "${LIMA_CIDATA_USER}")
		chown "${LIMA_CIDATA_UID}:${gid}" "${mountpoint}"
	done
fi

# Install or update the guestagent binary
mkdir -p "${LIMA_CIDATA_GUEST_INSTALL_PREFIX}"/bin
guestagent_updated=false
if diff -q "${LIMA_CIDATA_MNT}"/lima-guestagent "${LIMA_CIDATA_GUEST_INSTALL_PREFIX}"/bin/lima-guestagent 2>/dev/null; then
	echo "${LIMA_CIDATA_GUEST_INSTALL_PREFIX}/bin/lima-guestagent is up-to-date"
else
	install -m 755 "${LIMA_CIDATA_MNT}"/lima-guestagent "${LIMA_CIDATA_GUEST_INSTALL_PREFIX}"/bin/lima-guestagent
	guestagent_updated=true
fi

# Launch the guestagent service
if [ -f /sbin/openrc-run ]; then
	print_config() {
		# Convert .env to conf.d by wrapping values in double quotes.
		# Split the variable and value at the first "=" to handle cases where the value contains additional "=" characters.
		sed -E 's/^([^=]+)=(.*)/\1="\2"/' "${LIMA_CIDATA_MNT}/lima.env"
	}
	print_script() {
		# the openrc lima-guestagent service script
		cat <<-'EOF'
			#!/sbin/openrc-run
			supervisor=supervise-daemon

			log_file="${log_file:-/var/log/${RC_SVCNAME}.log}"
			err_file="${err_file:-${log_file}}"
			log_mode="${log_mode:-0644}"
			log_owner="${log_owner:-root:root}"

			supervise_daemon_args="${supervise_daemon_opts:---stderr \"${err_file}\" --stdout \"${log_file}\"}"

			name="lima-guestagent"
			description="Forward ports to the lima-hostagent"

			command=${LIMA_CIDATA_GUEST_INSTALL_PREFIX}/bin/lima-guestagent
			command_args="daemon --debug=${LIMA_CIDATA_DEBUG} --vsock-port \"${LIMA_CIDATA_VSOCK_PORT}\" --virtio-port \"${LIMA_CIDATA_VIRTIO_PORT}\""
			command_background=true
			pidfile="/run/lima-guestagent.pid"
		EOF
	}
	if [ "${guestagent_updated}" = "false" ] &&
		diff -q <(print_config) /etc/conf.d/lima-guestagent 2>/dev/null &&
		diff -q <(print_script) /etc/init.d/lima-guestagent 2>/dev/null; then
		echo "lima-guestagent service already up-to-date"
		exit 0
	fi

	print_config >/etc/conf.d/lima-guestagent
	print_script >/etc/init.d/lima-guestagent
	chmod 755 /etc/init.d/lima-guestagent

	rc-update add lima-guestagent default
	rc-service --ifstarted lima-guestagent restart # restart if running, otherwise do nothing
	rc-service --ifstopped lima-guestagent start   # start if not running, otherwise do nothing
else
	# Remove legacy systemd service
	rm -f "${LIMA_CIDATA_HOME}/.config/systemd/user/lima-guestagent.service"

	if [ "${LIMA_CIDATA_VSOCK_PORT}" != "0" ]; then
		sudo "${LIMA_CIDATA_GUEST_INSTALL_PREFIX}"/bin/lima-guestagent install-systemd --debug="${LIMA_CIDATA_DEBUG}" --guestagent-updated="${guestagent_updated}" --vsock-port "${LIMA_CIDATA_VSOCK_PORT}"
	elif [ "${LIMA_CIDATA_VIRTIO_PORT}" != "" ]; then
		sudo "${LIMA_CIDATA_GUEST_INSTALL_PREFIX}"/bin/lima-guestagent install-systemd --debug="${LIMA_CIDATA_DEBUG}" --guestagent-updated="${guestagent_updated}" --virtio-port "${LIMA_CIDATA_VIRTIO_PORT}"
	else
		sudo "${LIMA_CIDATA_GUEST_INSTALL_PREFIX}"/bin/lima-guestagent install-systemd --debug="${LIMA_CIDATA_DEBUG}" --guestagent-updated="${guestagent_updated}"
	fi
fi
