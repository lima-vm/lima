// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package plist

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"gotest.tools/v3/assert"

	"github.com/lima-vm/lima/v2/pkg/ptr"
)

func TestUnmarshalPlist(t *testing.T) {
	const input = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>arrKey</key>
	<array>
		<dict>
			<key>strKey</key>
			<string>strVal</string>
			<key> strKey with
			spaces</key>
			<string> strVal
			with space</string>
			<key>intKey</key>
			<integer>42</integer>
			<key> intKey with
			spaces</key>
			<integer>  43</integer>
			<key>intKeyZero</key>
			<integer>0</integer>
			<key>dataKey</key>
			<data>
			aGVs
			bG8=
			</data>
			<key>dateKey</key>
			<date>2020-01-02T03:04:05Z</date>
			<key>trueKey</key>
			<true/>
			<key>falseKey</key>
			<false/>
			<key>realKey</key>
			<real>3.14</real>
			<key>realExpKey</key>
			<real>1e-6</real>
		</dict>
	</array>
</dict>
</plist>
`
	expected := Plist{
		Value: Value{
			Dict: map[string]Value{
				"arrKey": {
					Array: Array{
						{
							Dict: map[string]Value{
								"strKey":                     {String: ptr.Of("strVal")},
								" strKey with\n\t\t\tspaces": {String: ptr.Of(" strVal\n\t\t\twith space")},
								"intKey":                     {Integer: ptr.Of(int64(42))},
								" intKey with\n\t\t\tspaces": {Integer: ptr.Of(int64(43))},
								"intKeyZero":                 {Integer: ptr.Of(int64(0))},
								"dataKey":                    {Data: []byte("hello")},
								"dateKey":                    {Date: ptr.Of(time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC))},
								"trueKey":                    {Boolean: ptr.Of(true)},
								"falseKey":                   {Boolean: ptr.Of(false)},
								"realKey":                    {Real: ptr.Of(3.14)},
								"realExpKey":                 {Real: ptr.Of(1e-6)},
							},
						},
					},
				},
			},
		},
	}

	plutilExpected := map[string]string{
		"arrKey.0.strKey":                     "strVal",
		"arrKey.0. strKey with\n\t\t\tspaces": " strVal\n\t\t\twith space",
		"arrKey.0.intKey":                     "42",
		"arrKey.0. intKey with\n\t\t\tspaces": "43",
		"arrKey.0.intKeyZero":                 "0",
		"arrKey.0.dataKey":                    "aGVsbG8=",
		"arrKey.0.dateKey":                    "2020-01-02T03:04:05Z",
		"arrKey.0.trueKey":                    "true",
		"arrKey.0.falseKey":                   "false",
		"arrKey.0.realKey":                    "3.140000",
		"arrKey.0.realExpKey":                 "0.000001",
	}

	var p Plist
	err := xml.Unmarshal([]byte(input), &p)
	assert.NilError(t, err)
	assert.DeepEqual(t, p, expected)

	if runtime.GOOS == "darwin" {
		ctx := t.Context()
		td := t.TempDir()
		testFilePath := filepath.Join(td, "test.plist")
		err := os.WriteFile(testFilePath, []byte(input), 0o644)
		assert.NilError(t, err)
		for plutilExpectedKey, plutilExpectedValue := range plutilExpected {
			plutilCmd := exec.CommandContext(ctx, "plutil", "-extract", plutilExpectedKey, "raw", "-n", "-o", "-", testFilePath)
			var stderr bytes.Buffer
			plutilCmd.Stderr = &stderr
			plutilOutput, err := plutilCmd.Output()
			assert.NilError(t, err, fmt.Sprintf("failed to run %v (stderr=%q)", plutilCmd.Args, stderr.String()))
			plutilOutputStr := string(plutilOutput)
			assert.Equal(t, plutilExpectedValue, plutilOutputStr, "plutil output mismatch for key %q", plutilExpectedKey)
		}
	}
}
