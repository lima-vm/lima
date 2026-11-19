#!/bin/sh

# SPDX-FileCopyrightText: Copyright The Lima Authors
# SPDX-License-Identifier: Apache-2.0

# Import the host USB devices this instance may use.
# The vz hostagent listens on the USB/IP port only when `usb` is enabled
# (`enabled: true`, or `devices` without `enabled: false`), so the probe is the switch.

set -eu

GA="${LIMA_CIDATA_GUEST_INSTALL_PREFIX}/bin/lima-guestagent"
"${GA}" usbip probe 2>/dev/null || exit 0

export DEBIAN_FRONTEND=noninteractive
# Ubuntu kernels like that of 24.04 keep vhci-hcd in linux-modules-extra.
# The timeout caps the update only, as killing the install could break dpkg. A slow link loses USB until next boot.
if ! modprobe vhci-hcd && ! {
	[ "${LIMA_CIDATA_SKIP_DEFAULT_DEPENDENCY_RESOLUTION}" != 1 ] &&
		grep -qs ubuntu /etc/os-release &&
		timeout 120 apt-get update &&
		apt-get install -y --no-upgrade --no-install-recommends -q "linux-modules-extra-$(uname -r)" &&
		modprobe vhci-hcd
}; then
	echo >&2 "WARNING: no vhci-hcd for kernel $(uname -r); USB passthrough will not work"
fi
"${GA}" usbip attach --all || echo >&2 "usbip: auto-attach failed"
