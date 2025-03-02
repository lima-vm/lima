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

package textutil

import (
	"bytes"
	"testing"
	"text/template"

	"gotest.tools/v3/assert"
)

func TestPrefixString(t *testing.T) {
	assert.Equal(t, "", PrefixString("- ", ""))
	assert.Equal(t, "\n", PrefixString("- ", "\n"))
	assert.Equal(t, "- foo", PrefixString("- ", "foo"))
	assert.Equal(t, "- foo\n- bar\n", PrefixString("- ", "foo\nbar\n"))
	assert.Equal(t, "- foo\n\n- bar\n", PrefixString("- ", "foo\n\nbar\n"))
}

func TestIndentString(t *testing.T) {
	assert.Equal(t, "  foo", IndentString(2, "foo"))
	assert.Equal(t, "  foo\n  bar\n", IndentString(2, "foo\nbar\n"))
}

func TestTrimString(t *testing.T) {
	assert.Equal(t, "foo", TrimString("\n", "foo"))
	assert.Equal(t, "bar", TrimString("\n", "bar\n"))
}

func TestMissingString(t *testing.T) {
	assert.Equal(t, "no", MissingString("no", ""))
	assert.Equal(t, "msg", MissingString("no", "msg"))
}

func TestTemplateFuncs(t *testing.T) {
	type X struct {
		Foo     int    `json:"foo" yaml:"foo"`
		Bar     string `json:"bar" yaml:"bar"`
		Message string `json:"message,omitempty" yaml:"message,omitempty"`
		Missing string `json:"missing,omitempty" yaml:"missing,omitempty"`
	}
	x := X{Foo: 42, Bar: "hello", Message: "One\nTwo\nThree", Missing: ""}

	testCases := map[string]string{
		"{{json .}}": `{"foo":42,"bar":"hello","message":"One\nTwo\nThree"}`,
		"{{yaml .}}": `---
foo: 42
bar: hello
message: |-
  One
  Two
  Three`,
		`{{.Bar}}{{"\n"}}{{.Message | missing "<no message>" | indent 2}}`: "hello\n  One\n  Two\n  Three",
		`{{.Message | indent}}`:  "  One\n  Two\n  Three",
		`{{.Missing | missing}}`: "<missing>",
	}

	for format, expected := range testCases {
		tmpl, err := template.New("format").Funcs(TemplateFuncMap).Parse(format)
		assert.NilError(t, err)
		var b bytes.Buffer
		assert.NilError(t, tmpl.Execute(&b, x))
		assert.Equal(t, expected, b.String())
	}
}
