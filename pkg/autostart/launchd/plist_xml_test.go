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

// TestPlistTemplatesSetExitTimeOut checks that both templates set ExitTimeOut. Without it
// launchd SIGKILLs the job 5 seconds after SIGTERM, which cuts a vz guest off mid-shutdown.
// TestPlistExitTimeOutExceedsGracefulStopTimeout in pkg/driver/vz checks the value against
// the driver's stop timeout.
func TestPlistTemplatesSetExitTimeOut(t *testing.T) {
	vars := map[string]string{
		"Instance": "default",
		"UserName": "user",
		"LimaHome": "/Users/user/.lima",
		"Binary":   "/usr/local/bin/limactl",
		"WorkDir":  "/Users/user/.lima/default",
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
			var key string
			found := false
			for {
				tok, err := dec.Token()
				if errors.Is(err, io.EOF) {
					break
				}
				assert.NilError(t, err)
				se, ok := tok.(xml.StartElement)
				if !ok {
					continue
				}
				switch se.Name.Local {
				case "key":
					assert.NilError(t, dec.DecodeElement(&key, &se))
				case "integer":
					var v int
					assert.NilError(t, dec.DecodeElement(&v, &se))
					if key == "ExitTimeOut" {
						found = true
						assert.Assert(t, v > 0, "ExitTimeOut must be positive, got %d", v)
					}
				}
			}
			assert.Assert(t, found, "plist has no ExitTimeOut:\n%s", out)
		})
	}
}
