#!/bin/sh

# SPDX-FileCopyrightText: Copyright The Lima Authors
# SPDX-License-Identifier: Apache-2.0

if command -v systemd-detect-virt >/dev/null 2>&1 && systemd-detect-virt --container >/dev/null 2>&1; then
	if command -v systemctl >/dev/null 2>&1; then
		systemctl mask systemd-networkd-wait-online.service
		systemctl mask NetworkManager-wait-online.service
		systemctl mask systemd-logind.service
		systemctl stop systemd-networkd-wait-online.service || true
		systemctl stop NetworkManager-wait-online.service || true
		systemctl stop systemd-logind.service || true
	fi
fi
