#!/bin/bash

# SPDX-FileCopyrightText: Copyright The Lima Authors
# SPDX-License-Identifier: Apache-2.0

set -eux -o pipefail

# Do nothing on OpenRC systems (e.g., Alpine Linux)
if [[ -f /sbin/openrc-run ]]; then
	exit 0
fi

# It depends on systemd-resolved
command -v systemctl >/dev/null 2>&1 || exit 0
systemctl is-enabled -q systemd-resolved.service || exit 0
command -v resolvectl >/dev/null 2>&1 || exit 0

# Configure systemd-resolved to enable mDNS resolution globally
enable_mdns_conf_path=/etc/systemd/resolved.conf.d/00-lima-enable-mdns.conf
enable_mdns_conf_content="[Resolve]
MulticastDNS=yes
"
# Create /etc/systemd/resolved.conf.d/00-lima-enable-mdns.conf if its content is different
if ! diff -q <(echo "${enable_mdns_conf_content}") "${enable_mdns_conf_path}" >/dev/null 2>&1; then
	mkdir -p "$(dirname "${enable_mdns_conf_path}")"
	echo "${enable_mdns_conf_content}" >"${enable_mdns_conf_path}"
	systemctl daemon-reload
	systemctl restart systemd-resolved.service
fi

# On Ubuntu, systemd.network's configuration won't work.
# See: https://unix.stackexchange.com/a/652582
# So we need to enable mDNS per-link using resolvectl.
for iface in $(resolvectl status | sed -n -E 's/^Link +[0-9]+ \(([^)]+)\)/\1/p'); do
	# This setting is volatile and will be lost on reboot, so we need to set it every time
	resolvectl mdns "${iface}" yes
done
