// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package sshutil

import (
	"strings"
	"testing"

	"gotest.tools/v3/assert"
)

func TestFormatConfig(t *testing.T) {
	var b strings.Builder
	opts := []string{"User=lima", "Hostname=127.0.0.1", "Port=60022"}
	err := Format(&b, "ssh", "default", FormatConfig, opts)
	assert.NilError(t, err)
	got := b.String()
	assert.Assert(t, strings.Contains(got, "Host lima-default\n"))
	assert.Assert(t, strings.Contains(got, "  User lima\n"))
	assert.Assert(t, strings.Contains(got, "  Port 60022\n"))
}

func TestFormatRejectsLineBreakInOption(t *testing.T) {
	for _, format := range Formats {
		var b strings.Builder
		opts := []string{"User=lima\n  ProxyCommand touch /tmp/pwned", "Port=60022"}
		err := Format(&b, "ssh", "default", format, opts)
		assert.ErrorContains(t, err, "line break")
		assert.Assert(t, !strings.Contains(b.String(), "ProxyCommand"), "format %q leaked an injected directive", format)
	}
}
