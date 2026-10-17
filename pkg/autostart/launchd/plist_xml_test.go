// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package launchd

import (
	"bytes"
	"encoding/xml"
	"io"
	"strings"
	"testing"

	"github.com/lima-vm/lima/v2/pkg/textutil"
)

// TestPlistTemplatesEscapeDynamicValues renders both launchd plist templates with values that
// contain XML metacharacters and checks that the result is well-formed XML whose decoded values
// are unchanged. Both templates are rendered through textutil.ExecuteTemplate, the same path the
// CLI's renderDaemonPlist and the autostart manager's renderTemplate take.
func TestPlistTemplatesEscapeDynamicValues(t *testing.T) {
	vars := map[string]string{
		"Instance":  `inst&<>"'`,
		"UserName":  `user&<>`,
		"LimaHome":  `/Users/a/R&D/<home>`,
		"Binary":    `/opt/a&b/<lima>/limactl`,
		"WorkDir":   `/work/a&'b"/<dir>`,
		"Path":      `/usr/bin:/opt/a&b:/r<s>`,
		"KeepAlive": "true",
	}
	for name, tmpl := range map[string]string{"LaunchAgent": Template, "LaunchDaemon": DaemonTemplate} {
		t.Run(name, func(t *testing.T) {
			out, err := textutil.ExecuteTemplate(tmpl, vars)
			if err != nil {
				t.Fatalf("render: %v", err)
			}
			decoded := strings.Join(decodeCharData(t, out), "\n")
			for _, key := range []string{"Instance", "LimaHome", "Binary", "WorkDir", "Path"} {
				if !strings.Contains(decoded, vars[key]) {
					t.Errorf("%s value %q did not round-trip through the rendered plist", key, vars[key])
				}
			}
		})
	}
}

// decodeCharData returns the character data of an XML document, failing the test if the document
// is not well-formed. The decoder unescapes entities, so a value that was escaped correctly comes
// back identical to the input; an unescaped '&' or '<' makes the document fail to parse.
func decodeCharData(t *testing.T, doc []byte) []string {
	t.Helper()
	dec := xml.NewDecoder(bytes.NewReader(doc))
	var out []string
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("rendered plist is not well-formed XML: %v\n%s", err, doc)
		}
		if cd, ok := tok.(xml.CharData); ok {
			if s := strings.TrimSpace(string(cd)); s != "" {
				out = append(out, s)
			}
		}
	}
	return out
}
