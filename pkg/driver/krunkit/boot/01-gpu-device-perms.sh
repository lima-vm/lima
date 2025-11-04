#!/bin/bash

# SPDX-FileCopyrightText: Copyright The Lima Authors
# SPDX-License-Identifier: Apache-2.0

set -eux -o pipefail

# Make DRM render/card nodes world-accessible
install -d -m 0755 /etc/udev/rules.d
cat >/etc/udev/rules.d/70-lima-drm.rules <<'EOF'
KERNEL=="render[D]*", SUBSYSTEM=="drm", MODE="0666"
KERNEL=="card*", SUBSYSTEM=="drm", MODE="0666"
EOF

# Apply to existing nodes now and future ones via udev
udevadm control --reload || true
udevadm trigger --subsystem-match=drm || true

if [ -d /dev/dri ]; then
	chmod 0666 /dev/dri/render[D]* 2>/dev/null || true
	chmod 0666 /dev/dri/card* 2>/dev/null || true
fi
