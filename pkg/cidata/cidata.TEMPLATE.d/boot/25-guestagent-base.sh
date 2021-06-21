#!/bin/sh
set -eux

# Create mount points
# NOTE: Busybox sh does not support `for ((i=0;i<$N;i++))` form
for f in $(seq 0 $((LIMA_CIDATA_MOUNTS - 1))); do
  mountpointvar="LIMA_CIDATA_MOUNTS_${f}_MOUNTPOINT"
  mountpoint="$(eval echo \$$mountpointvar)"
  mkdir -p "${mountpoint}"
  chown "${LIMA_CIDATA_USER}" "${mountpoint}"
done

# Install or update the guestagent binary
install -m 755 "${LIMA_CIDATA_MNT}"/lima-guestagent /usr/local/bin/lima-guestagent

# Launch the guestagent service
if [ -f /etc/alpine-release ]; then
  rc-update add lima-guestagent default
  rc-service lima-guestagent start
else
  until [ -e "/run/user/${LIMA_CIDATA_UID}/systemd/private" ]; do sleep 3; done
  sudo -iu "${LIMA_CIDATA_USER}" "XDG_RUNTIME_DIR=/run/user/${LIMA_CIDATA_UID}" lima-guestagent install-systemd
fi
