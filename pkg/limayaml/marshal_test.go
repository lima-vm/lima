// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package limayaml

import (
	"testing"

	"gotest.tools/v3/assert"

	"github.com/lima-vm/lima/v2/pkg/limatype"
	"github.com/lima-vm/lima/v2/pkg/ptr"
)

func TestMarshalEmpty(t *testing.T) {
	_, err := Marshal(&limatype.LimaYAML{}, false)
	assert.NilError(t, err)
}

func TestMarshalTilde(t *testing.T) {
	y := limatype.LimaYAML{
		Mounts: []limatype.Mount{
			{Location: "~", Writable: ptr.Of(false)},
			{Location: "/tmp/lima", Writable: ptr.Of(true)},
			{Location: "null"},
		},
	}
	b, err := Marshal(&y, true)
	assert.NilError(t, err)
	// yaml will load ~ (or null) as null
	// make sure that it is always quoted
	assert.Equal(t, string(b), `---
mounts:
- location: "~"
  writable: false
- location: /tmp/lima
  writable: true
- location: "null"
...
`)
}
