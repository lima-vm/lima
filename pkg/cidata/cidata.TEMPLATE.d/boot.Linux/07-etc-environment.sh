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

if command -v loginctl >/dev/null 2>&1 && [ "${orig}" != "$(cat /etc/environment)" ]; then
	loginctl terminate-user "${LIMA_CIDATA_USER}" || true
fi

# Signal that provisioning is done. The instance-id in the meta-data file changes on every boot,
# so any copy from a previous boot cycle will have different content.
cp "${LIMA_CIDATA_MNT}"/meta-data /run/lima-ssh-ready
