package textutil

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"text/template"

	"github.com/goccy/go-yaml"
)

// ExecuteTemplate executes a text/template template.
func ExecuteTemplate(tmpl string, args interface{}) ([]byte, error) {
	x, err := template.New("").Parse(tmpl)
	if err != nil {
		return nil, err
	}
	var b bytes.Buffer
	if err := x.Execute(&b, args); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

// PrefixString adds prefix to beginning of each line
func PrefixString(prefix string, text string) string {
	result := ""
	for _, line := range strings.Split(text, "\n") {
		if line == "" {
			continue
		}
		result += prefix + line + "\n"
	}
	return result
}

// IndentString add spaces to beginning of each line
func IndentString(size int, text string) string {
	prefix := strings.Repeat(" ", size)
	return PrefixString(prefix, text)
}

// TrimString removes characters from beginning and end
func TrimString(cutset string, text string) string {
	return strings.Trim(text, cutset)
}

// MissingString returns message if the text is empty
func MissingString(message string, text string) string {
	if text == "" {
		return message
	}
	return text
}

// TemplateFuncMap is a text/template FuncMap.
var TemplateFuncMap = template.FuncMap{
	"json": func(v interface{}) string {
		var b bytes.Buffer
		enc := json.NewEncoder(&b)
		enc.SetEscapeHTML(false)
		if err := enc.Encode(v); err != nil {
			panic(fmt.Errorf("failed to marshal as JSON: %+v: %w", v, err))
		}
		return strings.TrimSuffix(b.String(), "\n")
	},
	"yaml": func(v interface{}) string {
		var b bytes.Buffer
		enc := yaml.NewEncoder(&b)
		if err := enc.Encode(v); err != nil {
			panic(fmt.Errorf("failed to marshal as YAML: %+v: %w", v, err))
		}
		return "---\n" + strings.TrimSuffix(b.String(), "\n")
	},
	"indent":  IndentString,
	"trim":    TrimString,
	"missing": MissingString,
}

// TemplateFuncHelp is help for TemplateFuncMap.
var FuncHelp = []string{
	"indent <size>: add spaces to beginning of each line",
	"trim <cutset>: remove characters from beginning and end",
	"missing <message>: return message if the text is empty",
}
