package osutil

import (
	"strings"
	"testing"

	"gotest.tools/v3/assert"
)

func TestMachineID(t *testing.T) {
	t.Log(MachineID())
}

func TestParseIOPlatformUUIDFromIOPlatformExpertDevice(t *testing.T) {
	ioPlatformExpertDevice := `
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
        <key>IOObjectClass</key>
        <string>IORegistryEntry</string>
        <key>IORegistryEntryChildren</key>
        <array>
                <dict>
                        <key>foo</key>
                        <string>foo value</string>
                        <key>IOPlatformUUID</key>
                        <string>1A008DA1-06E0-49AB-8EC9-88E9C85F67FB</string>
                        <key>bar</key>
                        <string>bar value</string>
                </dict>
        </array>
        <key>IORegistryEntryName</key>
        <string>Root</string>
</dict>
</plist>
`
	got, err := parseIOPlatformUUIDFromIOPlatformExpertDevice(strings.NewReader(ioPlatformExpertDevice))
	assert.NilError(t, err)
	assert.Equal(t, "1A008DA1-06E0-49AB-8EC9-88E9C85F67FB", got)
}
