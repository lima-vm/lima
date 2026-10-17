// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"strings"
	"testing"

	"gotest.tools/v3/assert"
)

// TestRenderDaemonPlist checks that the plist carries the real LIMA_HOME and PATH. A missing
// key fails the render only where the template always prints it, and PATH is printed only
// when it is set, so the test looks for PATH itself.
func TestRenderDaemonPlist(t *testing.T) {
	t.Setenv("LIMA_HOME", "/some/lima/home")
	t.Setenv("PATH", "/Users/alice/R&D/bin:/usr/bin:/bin")
	for _, keepAlive := range []bool{false, true} {
		got, err := renderDaemonPlist("/limactl", "colima", "/some/path", "containerserver", keepAlive)
		assert.NilError(t, err)
		plist := string(got)
		assert.Assert(t, !strings.Contains(plist, "<no value>"),
			"plist has an unresolved template key:\n%s", plist)
		assert.Assert(t, strings.Contains(plist, "<key>LIMA_HOME</key>\n\t\t<string>/some/lima/home</string>"),
			"plist does not carry LIMA_HOME:\n%s", plist)
		assert.Assert(t, strings.Contains(plist, "<key>PATH</key>\n\t\t<string>/Users/alice/R&amp;D/bin:/usr/bin:/bin</string>"),
			"plist does not carry PATH:\n%s", plist)
		assert.Assert(t, strings.Contains(plist, "<string>containerserver</string>"))
		assert.Equal(t, strings.Contains(plist, "<key>KeepAlive</key>"), keepAlive)
	}
}
