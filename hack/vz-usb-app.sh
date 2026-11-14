#!/usr/bin/env bash

# SPDX-FileCopyrightText: Copyright The Lima Authors
# SPDX-License-Identifier: Apache-2.0

# Wraps lima-driver-vz in a signed app bundle for USB passthrough, leaving a
# symlink at the original path. See website/content/en/docs/config/usb.md.

set -eu -o pipefail

if [ "$#" -ne 3 ] || [ -z "$2" ] || [ -z "$3" ]; then
	echo >&2 "Usage: $0 LIBEXEC_DIR PROVISIONING_PROFILE SIGNING_IDENTITY"
	echo >&2 "(make vz-usb-app VZ_USB_PROVISIONING_PROFILE=... VZ_USB_SIGNING_IDENTITY=...)"
	exit 1
fi
libexec="$1"
profile="$2"
identity="$3"

entitlement="com.apple.developer.accessory-access.usb"
name="lima-driver-vz"
driver="${libexec}/${name}"
app="${libexec}/${name}.app"
repo_root="$(cd "$(dirname "$0")/.." && pwd)"

sdk_version="$(xcrun --show-sdk-version)"
if [ "${sdk_version%%.*}" -lt 27 ]; then
	echo >&2 "error: USB passthrough requires building with the macOS 27 SDK or later, got ${sdk_version}"
	exit 1
fi

tmp="$(mktemp -d)"
trap 'rm -rf "${tmp}"' EXIT

# The App ID and the entitlements must match the provisioning profile.
security cms -D -i "${profile}" >"${tmp}/profile.plist"
plist() {
	/usr/libexec/PlistBuddy -c "Print :$1" "${tmp}/profile.plist" 2>/dev/null
}
if [ "$(plist "Entitlements:${entitlement}")" != "true" ]; then
	echo >&2 "error: ${profile} does not grant the ${entitlement} entitlement"
	exit 1
fi
team_id="$(plist "TeamIdentifier:0")"
app_id="$(plist "Entitlements:com.apple.application-identifier")"
bundle_id="${app_id#"${team_id}".}"
case "${bundle_id}" in
*'*'*)
	echo >&2 "error: ${profile} is for the wildcard App ID ${app_id}; use a profile for an explicit App ID"
	exit 1
	;;
esac

# A rebuilt driver replaces the symlink left by a previous run.
if [ -L "${driver}" ] || [ ! -f "${driver}" ]; then
	echo >&2 "error: ${driver} is not a freshly built binary; run \`make ADDITIONAL_DRIVERS=vz limactl additional-drivers\` first"
	exit 1
fi

rm -rf "${app}"
mkdir -p "${app}/Contents/MacOS"
mv "${driver}" "${app}/Contents/MacOS/${name}"
cp "${profile}" "${app}/Contents/embedded.provisionprofile"
cat >"${app}/Contents/Info.plist" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CFBundleExecutable</key>
	<string>${name}</string>
	<key>CFBundleIdentifier</key>
	<string>${bundle_id}</string>
	<key>CFBundleName</key>
	<string>Lima</string>
	<key>CFBundlePackageType</key>
	<string>APPL</string>
	<key>LSMinimumSystemVersion</key>
	<string>13.0</string>
	<key>NSPrincipalClass</key>
	<string>NSApplication</string>
</dict>
</plist>
EOF

# Without com.apple.security.device.usb, macOS discloses no USB accessory,
# even to an unsandboxed process.
cp "${repo_root}/vz.entitlements" "${tmp}/entitlements.plist"
/usr/libexec/PlistBuddy \
	-c "Add :${entitlement} bool true" \
	-c "Add :com.apple.security.device.usb bool true" \
	-c "Add :com.apple.application-identifier string ${app_id}" \
	-c "Add :com.apple.developer.team-identifier string ${team_id}" \
	"${tmp}/entitlements.plist"
# Development profiles grant get-task-allow, Developer ID profiles do not.
if [ "$(plist "Entitlements:com.apple.security.get-task-allow")" = "true" ]; then
	/usr/libexec/PlistBuddy -c "Add :com.apple.security.get-task-allow bool true" "${tmp}/entitlements.plist"
fi

# For notarization: VZ_USB_CODESIGN_FLAGS="--options runtime --timestamp".
# shellcheck disable=SC2086 # intentional word splitting
codesign --force ${VZ_USB_CODESIGN_FLAGS:---timestamp=none} \
	--entitlements "${tmp}/entitlements.plist" --sign "${identity}" "${app}"
codesign --verify --strict --verbose=2 "${app}"

ln -s "${name}.app/Contents/MacOS/${name}" "${driver}"
echo "Signed ${app} (${app_id}) with the ${entitlement} entitlement"
