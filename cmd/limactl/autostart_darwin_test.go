// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"strings"
	"testing"

	"gotest.tools/v3/assert"
)

// The daemon plist is rendered from a map, and text/template turns a key the template
// references but the map omits into the literal string "<no value>" instead of failing.
// That would be written into /Library/LaunchDaemons verbatim, so assert that the rendered
// plist carries the real LIMA_HOME and no placeholder.
func TestRenderDaemonPlist(t *testing.T) {
	t.Setenv("LIMA_HOME", "/some/lima/home")
	for _, keepAlive := range []bool{false, true} {
		got, err := renderDaemonPlist("/limactl", "colima", "/some/path", "containerserver", keepAlive)
		assert.NilError(t, err)
		plist := string(got)
		assert.Assert(t, !strings.Contains(plist, "<no value>"),
			"plist has an unresolved template key:\n%s", plist)
		assert.Assert(t, strings.Contains(plist, "<key>LIMA_HOME</key>\n\t\t<string>/some/lima/home</string>"),
			"plist does not carry LIMA_HOME:\n%s", plist)
		assert.Assert(t, strings.Contains(plist, "<string>containerserver</string>"))
		assert.Equal(t, strings.Contains(plist, "<key>KeepAlive</key>"), keepAlive)
	}
}
