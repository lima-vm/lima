/*
Copyright The Lima Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package yqutil

import (
	"strings"
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
	// Note: yq currently removes empty lines, but not comments
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

func TestEvaluateMergeExpression(t *testing.T) {
	expression := `select(di == 0) * select(di == 1)`
	content := `
yolo: true
foo:
  bar: 1
  baz: 2
---
foo:
  bar: 3
  fomo: false
`
	expected := `
yolo: true
foo:
  bar: 3
  baz: 2
  fomo: false
`
	out, err := EvaluateExpression(expression, []byte(strings.TrimSpace(content)))
	assert.NilError(t, err)
	assert.Equal(t, strings.TrimSpace(expected), strings.TrimSpace(string(out)))
}

func TestJoin(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected string
	}{
		{
			name:     "multiple values",
			input:    []string{"foo", "bar", "baz"},
			expected: "foo | bar | baz",
		},
		{
			name:     "one value",
			input:    []string{"foo"},
			expected: "foo",
		},
		{
			name:     "empty values",
			input:    []string{},
			expected: "",
		},
		{
			name:     "nil values",
			input:    nil,
			expected: "",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual := Join(test.input)
			assert.Equal(t, test.expected, actual)
		})
	}
}
