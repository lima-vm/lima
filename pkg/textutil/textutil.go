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
}
