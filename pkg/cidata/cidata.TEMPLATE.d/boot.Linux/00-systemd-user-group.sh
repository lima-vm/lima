#!/bin/sh

# SPDX-FileCopyrightText: Copyright The Lima Authors
# SPDX-License-Identifier: Apache-2.0

set -eu

# Add the user to the "systemd-journal" group so that `journalctl --user` works
# out of the box for UIDs in 1...999 ("system users"), without failing with
# "No journal files were opened due to insufficient permissions".
# https://github.com/lima-vm/lima/issues/5047
# The membership is needed for any UID because the hostagent reads the system
# journal of cloud-init-main.service without sudo for `limactl start --progress`.
# https://github.com/lima-vm/lima/issues/5758
# The "systemd-journal" group only exists on systemd-based distributions,
# so this is a no-op elsewhere.
getent group systemd-journal >/dev/null 2>&1 || exit 0

usermod -aG systemd-journal "${LIMA_CIDATA_USER}"
