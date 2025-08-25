// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package limayaml

import (
	"testing"

	"github.com/goccy/go-yaml"
	"gotest.tools/v3/assert"

	"github.com/lima-vm/lima/v2/pkg/ptr"
)

func TestMarshalEmpty(t *testing.T) {
	_, err := Marshal(&LimaYAML{}, false)
	assert.NilError(t, err)
}

func TestMarshalTilde(t *testing.T) {
	y := LimaYAML{
		Mounts: []Mount{
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

type Opts struct {
	Foo int
	Bar string
}

var (
	opts = Opts{Foo: 1, Bar: "two"}
	text = `{"foo":1,"bar":"two"}`
	code any
)

func TestConvert(t *testing.T) {
	err := yaml.Unmarshal([]byte(text), &code)
	assert.NilError(t, err)
	o := opts
	var a any
	err = Convert(o, &a, "")
	assert.NilError(t, err)
	assert.DeepEqual(t, a, code)
	err = Convert(a, &o, "")
	assert.NilError(t, err)
	assert.Equal(t, o, opts)
}

func TestQEMUOpts(t *testing.T) {
	text := `
vmType: "qemu"
vmOpts:
  qemu:
    minimumVersion: null
    cpuType:
`
	var y LimaYAML
	err := Unmarshal([]byte(text), &y, "lima.yaml")
	assert.NilError(t, err)
	var o QEMUOpts
	err = Convert(y.VMOpts[QEMU], &o, QEMU)
	assert.NilError(t, err)
}
