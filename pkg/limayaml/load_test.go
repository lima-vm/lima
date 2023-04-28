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
}

func TestLoadDiskStruct(t *testing.T) {
	s := `
additionalDisks:
- name: "name"
`
	y, err := Load([]byte(s), "disk.yaml")
	assert.NilError(t, err)
	assert.Assert(t, len(y.AdditionalDisks) == 1)
	assert.Equal(t, y.AdditionalDisks[0].Name, "name")
}
