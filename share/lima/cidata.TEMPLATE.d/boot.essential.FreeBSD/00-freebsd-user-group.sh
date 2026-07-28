#!/bin/sh

# SPDX-FileCopyrightText: Copyright The Lima Authors
# SPDX-License-Identifier: Apache-2.0

# This script ensures that the user is created with the expected UID.
# This script is needed because nuageinit does not create the user with the expected UID.

set -eu

[ "$(stat -f %u "${LIMA_CIDATA_HOME}")" = "${LIMA_CIDATA_UID}" ] && exit 0

pw usermod -n "${LIMA_CIDATA_USER}" -u "${LIMA_CIDATA_UID}"
gid="$(id -g "${LIMA_CIDATA_USER}")"
chown -R "${LIMA_CIDATA_UID}:${gid}" "${LIMA_CIDATA_HOME}"
