#!/bin/sh

# SPDX-FileCopyrightText: Copyright The Lima Authors
# SPDX-License-Identifier: Apache-2.0

set -eu

# Add the user to the "systemd-journal" group so that `journalctl`
# works out of the box without sudo (e.g. for `journalctl --user` on macOS UID 501,
# and for streaming system unit logs such as `journalctl -u cloud-init-main.service`
# in `limactl start --progress`).
# https://github.com/lima-vm/lima/issues/5047
# https://github.com/lima-vm/lima/issues/5758
# The "systemd-journal" group only exists on systemd-based distributions,
# so this is a no-op elsewhere.
getent group systemd-journal >/dev/null 2>&1 || exit 0
id "${LIMA_CIDATA_USER}" >/dev/null 2>&1 || exit 0

usermod -aG systemd-journal "${LIMA_CIDATA_USER}"
