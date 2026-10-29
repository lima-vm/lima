// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package launchd

import (
	"bytes"
	"encoding/xml"
	"errors"
	"io"
	"testing"

	"gotest.tools/v3/assert"

	"github.com/lima-vm/lima/v2/pkg/textutil"
)

// TestPlistTemplatesEscapeDynamicValues renders both templates with XML metacharacters in every
// value. A value printed without "xml" leaves a raw & or < and makes the plist malformed, so the
// test decodes the whole document.
func TestPlistTemplatesEscapeDynamicValues(t *testing.T) {
	vars := map[string]string{
		"Instance":  `inst&<>`,
		"UserName":  `user&<>`,
		"LimaHome":  `/Users/a/R&D/<home>`,
		"Binary":    `/opt/a&b/<lima>/limactl`,
		"WorkDir":   `/work/a&<>/dir`,
		"Path":      `/usr/bin:/opt/a&b:/r<s>`,
		"KeepAlive": "true",
	}
	for _, tt := range []struct {
		name string
		tmpl string
	}{
		{"LaunchAgent", Template},
		{"LaunchDaemon", DaemonTemplate},
	} {
		t.Run(tt.name, func(t *testing.T) {
			out, err := textutil.ExecuteTemplate(tt.tmpl, vars)
			assert.NilError(t, err)
			dec := xml.NewDecoder(bytes.NewReader(out))
			for {
				_, err := dec.Token()
				if errors.Is(err, io.EOF) {
					break
				}
				assert.NilError(t, err, "plist is not well-formed XML:\n%s", out)
			}
			assert.Assert(t, bytes.Contains(out, []byte("R&amp;D")),
				"LIMA_HOME ampersand is not escaped:\n%s", out)
		})
	}
}

// TestPlistTemplatesOmitOptionalValues checks that a map without Path and KeepAlive renders
// without error and leaves the PATH and KeepAlive entries out.
func TestPlistTemplatesOmitOptionalValues(t *testing.T) {
	vars := map[string]string{
		"Instance": "foo",
		"UserName": "alice",
		"LimaHome": "/home/alice/.lima",
		"Binary":   "/usr/local/bin/limactl",
		"WorkDir":  "/home/alice",
	}
	for _, tt := range []struct {
		name string
		tmpl string
	}{
		{"LaunchAgent", Template},
		{"LaunchDaemon", DaemonTemplate},
	} {
		t.Run(tt.name, func(t *testing.T) {
			out, err := textutil.ExecuteTemplate(tt.tmpl, vars)
			assert.NilError(t, err)
			assert.Assert(t, !bytes.Contains(out, []byte("<key>PATH</key>")), "PATH should be omitted when Path is unset")
			assert.Assert(t, !bytes.Contains(out, []byte("<key>KeepAlive</key>")), "KeepAlive should be omitted when unset")
		})
	}
}
