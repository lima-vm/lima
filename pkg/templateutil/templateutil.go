package templateutil

import (
	"bytes"
	"text/template"
)

func Execute(tmpl string, args interface{}) ([]byte, error) {
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
