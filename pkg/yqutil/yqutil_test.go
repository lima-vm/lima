package yqutil

import (
	"testing"

	"gotest.tools/v3/assert"
)

func TestValidateContent(t *testing.T) {
	content := `
# comment
foo: bar
`
	err := ValidateContent([]byte(content))
	assert.NilError(t, err)
}

func TestValidateContentError(t *testing.T) {
	content := `
- foo: bar
  foo
  bar
`
	err := ValidateContent([]byte(content))
	assert.ErrorContains(t, err, "could not find expected")
}

func TestEvaluateExpressionEmpty(t *testing.T) {
	expression := ""
	content := `
foo: bar
`
	expected := content
	out, err := EvaluateExpression(expression, []byte(content))
	assert.NilError(t, err)
	assert.Equal(t, expected, string(out))
}

func TestEvaluateExpressionSimple(t *testing.T) {
	expression := `.cpus = 2 | .memory = "2GiB"`
	content := `
# CPUs
cpus: null

# Memory size
memory: null
`
	// Note: yamlfmt will fix removal of empty lines
	expected := `
# CPUs
cpus: 2

# Memory size
memory: 2GiB
`
	out, err := EvaluateExpression(expression, []byte(content))
	assert.NilError(t, err)
	assert.Equal(t, expected, string(out))
}

func TestEvaluateExpressionComplex(t *testing.T) {
	expression := `.mounts += {"location": "foo", "mountPoint": "bar"}`
	content := `
# Expose host directories to the guest, the mount point might be accessible from all UIDs in the guest
# 🟢 Builtin default: null (Mount nothing)
# 🔵 This file: Mount the home as read-only, /tmp/lima as writable
mounts:
- location: "~"
  # Configure the mountPoint inside the guest.
  # 🟢 Builtin default: value of location
  mountPoint: null
`
	// Note: yq will use canonical yaml, with indented sequences
	// Note: yq will not explicitly quote strings, when not needed
	// Note: yamlfmt will fix indentation of sequences
	expected := `
# Expose host directories to the guest, the mount point might be accessible from all UIDs in the guest
# 🟢 Builtin default: null (Mount nothing)
# 🔵 This file: Mount the home as read-only, /tmp/lima as writable
mounts:
- location: "~"
  # Configure the mountPoint inside the guest.
  # 🟢 Builtin default: value of location
  mountPoint: null
- location: foo
  mountPoint: bar
`
	out, err := EvaluateExpression(expression, []byte(content))
	assert.NilError(t, err)
	assert.Equal(t, expected, string(out))
}

func TestEvaluateExpressionError(t *testing.T) {
	expression := `arch: aarch64`
	_, err := EvaluateExpression(expression, []byte(""))
	assert.ErrorContains(t, err, "invalid input text")
}
