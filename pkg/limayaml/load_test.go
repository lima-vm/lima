package limayaml

import (
	"testing"

	"gotest.tools/v3/assert"
)

func TestLoadEmpty(t *testing.T) {
	_, err := Load([]byte{}, "empty.yaml")
	assert.NilError(t, err)
}

func TestLoadError(t *testing.T) {
	// missing a "script:" line
	s := `
provision:
- mode: system
  script: |
    #!/bin/sh
    echo one
- mode: system
    #!/bin/sh
    echo two
- mode: system
  script: |
    #!/bin/sh
    echo three
`
	_, err := Load([]byte(s), "error.yaml")
	assert.ErrorContains(t, err, "failed to unmarshal YAML")
}

func TestLoadDiskString(t *testing.T) {
	s := `
additionalDisks:
- name
`
	y, err := Load([]byte(s), "disk.yaml")
	assert.NilError(t, err)
	assert.Equal(t, len(y.AdditionalDisks), 1)
	assert.Equal(t, y.AdditionalDisks[0].Name, "name")
	assert.Assert(t, y.AdditionalDisks[0].Format == nil)
	assert.Assert(t, y.AdditionalDisks[0].FSType == nil)
	assert.Assert(t, y.AdditionalDisks[0].FSArgs == nil)
}

func TestLoadDiskStruct(t *testing.T) {
	s := `
additionalDisks:
- name: "name"
  format: false
  fsType: "xfs"
  fsArgs: ["-i","size=512"]
`
	y, err := Load([]byte(s), "disk.yaml")
	assert.NilError(t, err)
	assert.Assert(t, len(y.AdditionalDisks) == 1)
	assert.Equal(t, y.AdditionalDisks[0].Name, "name")
	assert.Assert(t, y.AdditionalDisks[0].Format != nil)
	assert.Equal(t, *y.AdditionalDisks[0].Format, false)
	assert.Assert(t, y.AdditionalDisks[0].FSType != nil)
	assert.Equal(t, *y.AdditionalDisks[0].FSType, "xfs")
	assert.Assert(t, len(y.AdditionalDisks[0].FSArgs) == 2)
	assert.Equal(t, y.AdditionalDisks[0].FSArgs[0], "-i")
	assert.Equal(t, y.AdditionalDisks[0].FSArgs[1], "size=512")
}

func TestLoadHostProvisionWithStringHostOS(t *testing.T) {
	s := `
hostProvision:
- hostOS: linux
`
	y, err := Load([]byte(s), "hostprovision.yaml")
	assert.NilError(t, err)
	assert.Equal(t, len(y.HostProvision), 1)
	assert.Assert(t, y.HostProvision[0].HostOS != nil)
	assert.DeepEqual(t, *y.HostProvision[0].HostOS, StringArray{"linux"})
}

func TestLoadHostProvisionWithSliceHostOS(t *testing.T) {
	s := `
hostProvision:
- hostOS: [linux, darwin]
`
	y, err := Load([]byte(s), "hostprovision.yaml")
	assert.NilError(t, err)
	assert.Equal(t, len(y.HostProvision), 1)
	assert.Assert(t, y.HostProvision[0].HostOS != nil)
	assert.DeepEqual(t, *y.HostProvision[0].HostOS, StringArray{"linux", "darwin"})
}
