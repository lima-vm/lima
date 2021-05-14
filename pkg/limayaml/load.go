package limayaml

import (
	"gopkg.in/yaml.v2"
)

// Load loads the yaml and fulfills unspecified fields with the default values.
//
// Load does not validate. Use Validate for validation.
func Load(b []byte) (*LimaYAML, error) {
	var y LimaYAML
	if err := yaml.Unmarshal(b, &y); err != nil {
		return nil, err
	}
	FillDefault(&y)
	return &y, nil
}
