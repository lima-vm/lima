package textutil

import (
	"bytes"
	"testing"
	"text/template"

	"gotest.tools/v3/assert"
)

func TestPrefixString(t *testing.T) {
	assert.Equal(t, "- foo", PrefixString("- ", "foo"))
	assert.Equal(t, "- foo\n- bar\n", PrefixString("- ", "foo\nbar\n"))
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
	}
	x := X{Foo: 42, Bar: "hello", Message: "One\nTwo\nThree"}

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
	}

	for format, expected := range testCases {
		tmpl, err := template.New("format").Funcs(TemplateFuncMap).Parse(format)
		assert.NilError(t, err)
		var b bytes.Buffer
		assert.NilError(t, tmpl.Execute(&b, x))
		assert.Equal(t, expected, b.String())
	}
}
