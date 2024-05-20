package limayaml

import (
	"github.com/goccy/go-yaml"
)

func marshalString(s string) ([]byte, error) {
	if s == "null" || s == "~" {
		// work around go-yaml bugs
		return []byte("\"" + s + "\""), nil
	}
	return yaml.Marshal(s)
}

func marshalYAML(v interface{}) ([]byte, error) {
	options := []yaml.EncodeOption{yaml.CustomMarshaler[string](marshalString)}
	return yaml.MarshalWithOptions(v, options...)
}

// Save saves the yaml.
//
// Save does not fill defaults. Use FillDefaults.
func Save(y *LimaYAML) ([]byte, error) {
	return marshalYAML(y)
}
