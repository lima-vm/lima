#!/bin/sh

# SPDX-FileCopyrightText: Copyright The Lima Authors
# SPDX-License-Identifier: Apache-2.0

set -eux

# /etc/environment must be written after 04-persistent-data-volume.sh has run to
# make sure the changes on a restart are applied to the persisted version.

orig=$(test ! -f /etc/environment || cat /etc/environment)
if [ -e /etc/environment ]; then
	sed -i '/#LIMA-START/,/#LIMA-END/d' /etc/environment
fi
cat "${LIMA_CIDATA_MNT}/etc_environment" >>/etc/environment

# It is possible that a requirements script has started an ssh session before
# /etc/environment was updated, so we need to kill it to make sure it will
# restart with the updated environment before "linger" is being enabled.

opening_sessions() {
	for session in $(loginctl show-user "${LIMA_CIDATA_USER}" --property=Sessions --value 2>/dev/null); do
		if [ "$(loginctl show-session "${session}" --property=State --value 2>/dev/null)" = opening ]; then
			echo "${session}"
		fi
	done
}

if command -v loginctl >/dev/null 2>&1 && [ "${orig}" != "$(cat /etc/environment)" ]; then
	# systemd v255's logind spins forever, and every later login hangs, when
	# terminate-user stops a session that is still opening.
	# https://github.com/lima-vm/lima/issues/5570
	retry=0
	while [ -n "$(opening_sessions)" ] && [ "${retry}" -lt 30 ]; do
		retry=$((retry + 1))
		sleep 1
	done
	loginctl terminate-user "${LIMA_CIDATA_USER}" || true
fi

# Signal that provisioning is done. The instance ID changes on every boot,
# so any value from a previous boot cycle will be different.
echo "${LIMA_CIDATA_IID}" >/run/lima-ssh-ready
