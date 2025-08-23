// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package limayaml

import (
	"strings"
	"testing"
	"text/template"

	"github.com/goccy/go-yaml"
	"gotest.tools/v3/assert"

	"github.com/lima-vm/lima/v2/pkg/limatype"
	"github.com/lima-vm/lima/v2/pkg/ptr"
)

func dumpYAML(t *testing.T, d any) string {
	b, err := yaml.Marshal(d)
	assert.NilError(t, err)
	return string(b)
}

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

func TestVMOpts(t *testing.T) {
	text := `
vmType: null
`
	var y limatype.LimaYAML
	err := Unmarshal([]byte(text), &y, "lima.yaml")
	assert.NilError(t, err)
	var o limatype.VMOpts
	err = Convert(y.VMOpts, &o, "vmOpts")
	assert.NilError(t, err)
	t.Log(dumpYAML(t, o))
}

func TestQEMUOpts(t *testing.T) {
	text := `
vmType: "qemu"
vmOpts:
  qemu:
    minimumVersion: null
    cpuType:
`
	var y limatype.LimaYAML
	err := Unmarshal([]byte(text), &y, "lima.yaml")
	assert.NilError(t, err)
	var o limatype.QEMUOpts
	err = Convert(y.VMOpts[limatype.QEMU], &o, "vmOpts.qemu")
	assert.NilError(t, err)
	t.Log(dumpYAML(t, o))
}

func TestVZOpts(t *testing.T) {
	text := `
vmType: "vz"
vmOpts:
  vz:
    rosetta:
      enabled: null
      binfmt: null
`
	var y limatype.LimaYAML
	err := Unmarshal([]byte(text), &y, "lima.yaml")
	assert.NilError(t, err)
	var o limatype.VZOpts
	err = Convert(y.VMOpts[limatype.VZ], &o, "vmOpts.vz")
	assert.NilError(t, err)
	t.Log(dumpYAML(t, o))
}

func TestVMOptsNull(t *testing.T) {
	text := `
vmOpts: null
`
	var y limatype.LimaYAML
	err := Unmarshal([]byte(text), &y, "lima.yaml")
	assert.NilError(t, err)
	var o limatype.VMOpts
	err = Convert(y.VMOpts, &o, "vmOpts")
	assert.NilError(t, err)
	var oq limatype.QEMUOpts
	err = Convert(y.VMOpts[limatype.QEMU], &oq, "vmOpts.qemu")
	assert.NilError(t, err)
	var ov limatype.VZOpts
	err = Convert(y.VMOpts[limatype.VZ], &ov, "vmOpts.vz")
	assert.NilError(t, err)
}

type FormatData struct {
	limatype.Instance `yaml:",inline"`
}

func TestVZOptsRosettaMessage(t *testing.T) {
	text := `
vmType: "vz"
vmOpts:
  vz:
    rosetta:
      enabled: true
      binfmt: false

message: |
  {{- if .Instance.Config.VMOpts.vz.rosetta.enabled}}
  Rosetta is enabled in this VM, so you can run x86_64 containers on Apple Silicon.
  {{- end}}
`
	want := `vmType: vz
vmOpts:
  vz:
    rosetta:
      binfmt: false
      enabled: true
message: |
  
  Rosetta is enabled in this VM, so you can run x86_64 containers on Apple Silicon.
`
	var y limatype.LimaYAML
	err := Unmarshal([]byte(text), &y, "lima.yaml")
	assert.NilError(t, err)
	tmpl, err := template.New("format").Parse(y.Message)
	assert.NilError(t, err)
	inst := limatype.Instance{Config: &y}
	var message strings.Builder
	data := FormatData{Instance: inst}
	err = tmpl.Execute(&message, data)
	assert.NilError(t, err)
	y.Message = message.String()
	b, err := Marshal(&y, false)
	assert.NilError(t, err)
	assert.Equal(t, string(b), want)
}
