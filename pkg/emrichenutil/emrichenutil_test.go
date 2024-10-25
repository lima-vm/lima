package emrichenutil

import (
	"testing"

	"gotest.tools/v3/assert"
)

func TestEvaluateTemplate(t *testing.T) {
	content := `
# CPUs
cpus: 2

# Memory size
memory: 2GiB
`
	// Note: emrichen currently removes empty lines, but not comments
	expected :=
`# CPUs
cpus: 2
# Memory size
memory: 2GiB
`
	out, err := EvaluateTemplate([]byte(content))
	assert.NilError(t, err)
	assert.Equal(t, expected, string(out))
}

func TestEvaluateTemplateDefaults(t *testing.T) {
	content := `
!Defaults
var1: default1
var2: default2
---
var1: !Var var1
var2: !Var var2
`
	expected := `var1: default1
var2: default2
`
	out, err := EvaluateTemplate([]byte(content))
	assert.NilError(t, err)
	assert.Equal(t, expected, string(out))
}
