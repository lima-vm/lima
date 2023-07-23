#!/bin/sh

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
install -m 755 "${LIMA_CIDATA_MNT}"/lima-guestagent "${LIMA_CIDATA_GUEST_INSTALL_PREFIX}"/bin/lima-guestagent

# Launch the guestagent service
if [ -f /sbin/openrc-run ]; then
	# Install the openrc lima-guestagent service script
	cat >/etc/init.d/lima-guestagent <<'EOF'
#!/sbin/openrc-run
supervisor=supervise-daemon

name="lima-guestagent"
description="Forward ports to the lima-hostagent"

command=${LIMA_CIDATA_GUEST_INSTALL_PREFIX}/bin/lima-guestagent
command_args="daemon"
command_background=true
pidfile="/run/lima-guestagent.pid"
EOF
	chmod 755 /etc/init.d/lima-guestagent

	rc-update add lima-guestagent default
	rc-service lima-guestagent start
else
	# Remove legacy systemd service
	rm -f "/home/${LIMA_CIDATA_USER}.linux/.config/systemd/user/lima-guestagent.service"

	sudo "${LIMA_CIDATA_GUEST_INSTALL_PREFIX}"/bin/lima-guestagent install-systemd
fi
