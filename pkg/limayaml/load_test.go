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
