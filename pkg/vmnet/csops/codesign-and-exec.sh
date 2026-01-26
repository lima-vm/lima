#!/bin/bash

# SPDX-FileCopyrightText: Copyright The Lima Authors
# SPDX-License-Identifier: Apache-2.0

set -euo pipefail

# If the OS is macOS and codesign is available, sign ${1} as an executable
# with the virtualization entitlement, then exec it with the given arguments.
# Expected to be used with `-exec .../codesign-and-exec.sh` when executing `go` command.
if OS=$(uname -s) && [[ ${OS} == "Darwin" ]] && command -v codesign >/dev/null 2>&1; then
	cat <<-'EOF' >"$1.entitlements"
		<?xml version="1.0" encoding="UTF-8"?>
		<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
		<plist version="1.0">
		<dict>
			<key>com.apple.security.virtualization</key>
			<true/>
		</dict>
		</plist>
	EOF
	codesign --entitlements "$1.entitlements" --force -s - -v "$1"
	rm -f "$1.entitlements"
fi
exec "${@}"
