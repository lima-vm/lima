#!/bin/sh

# SPDX-FileCopyrightText: Copyright The Lima Authors
# SPDX-License-Identifier: Apache-2.0

set -eu

# If the UID is in 1...999 ("system users"), add the user to the "systemd-journal"
# group so that `journalctl --user` works out of the box, without failing with
# "No journal files were opened due to insufficient permissions".
# https://github.com/lima-vm/lima/issues/5047
# The "systemd-journal" group only exists on systemd-based distributions,
# so this is a no-op elsewhere.
[ "${LIMA_CIDATA_UID}" -le 1000 ] || exit 0
getent group systemd-journal >/dev/null 2>&1 || exit 0

usermod -aG systemd-journal "${LIMA_CIDATA_USER}"
