#!/bin/sh
# This script fixes the signature of QEMU binary with the "com.apple.security.hypervisor" entitlement.
#
# A workaround for "QEMU (homebrew) is broken on Intel: `[hostagent] Driver stopped due to error: "signal: abort trap"` ..."
#
# https://github.com/lima-vm/lima/issues/1742
# https://github.com/Homebrew/homebrew-core/issues/140244

set -eux

cat >entitlements.xml <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>com.apple.security.hypervisor</key>
    <true/>
</dict>
</plist>
EOF

codesign --sign - --entitlements entitlements.xml --force "$(which qemu-system-"$(uname -m | sed -e s/arm64/aarch64/)")"

rm -f entitlements.xml
