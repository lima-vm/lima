// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

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

func TestXMLFunc(t *testing.T) {
	xmlFunc := TemplateFuncMap["xml"].(func(string) (string, error))
	out, err := xmlFunc("a&<>'\"\t\n\r\uFFFD")
	assert.NilError(t, err)
	assert.Equal(t, "a&amp;&lt;&gt;&#39;&#34;&#x9;&#xA;&#xD;\uFFFD", out)

	for s, msg := range map[string]string{
		"a\xffb":         "not valid UTF-8",
		"a\xed\xa0\x80b": "not valid UTF-8", // an encoded surrogate
		"a\x01b":         "character U+0001 not allowed in XML 1.0",
		"a\uFFFEb":       "character U+FFFE not allowed in XML 1.0",
	} {
		_, err := xmlFunc(s)
		assert.ErrorContains(t, err, msg, "%q", s)
	}
}
