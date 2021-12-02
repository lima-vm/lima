package limayaml

import (
	"gopkg.in/yaml.v2"
)

// Save saves the yaml.
//
// Save does not validate. Use Validate for validation.
func Save(y *LimaYAML) ([]byte, error) {
	b, err := yaml.Marshal(y)
	if err != nil {
		return nil, err
	}
	return b, nil
}
