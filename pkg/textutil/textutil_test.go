package textutil

import (
	"bytes"
	"testing"
	"text/template"

	"gotest.tools/v3/assert"
)

func TestTemplateFuncs(t *testing.T) {
	type X struct {
		Foo int    `json:"foo" yaml:"foo"`
		Bar string `json:"bar" yaml:"bar"`
	}
	x := X{Foo: 42, Bar: "hello"}

	testCases := map[string]string{
		"{{json .}}": `{"foo":42,"bar":"hello"}`,
		"{{yaml .}}": `---
foo: 42
bar: hello`,
	}

	for format, expected := range testCases {
		tmpl, err := template.New("format").Funcs(TemplateFuncMap).Parse(format)
		assert.NilError(t, err)
		var b bytes.Buffer
		assert.NilError(t, tmpl.Execute(&b, x))
		assert.Equal(t, expected, b.String())
	}
}
