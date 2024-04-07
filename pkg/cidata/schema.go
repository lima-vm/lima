package cidata

import (
	"bytes"
	_ "embed"
	"fmt"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v5"
	"gopkg.in/yaml.v3"
)

// schemaURL is the identifier, not the context
const schemaURL = "https://raw.githubusercontent.com/canonical/cloud-init/main/cloudinit/config/schemas/schema-cloud-config-v1.json"

//go:embed schemas/schema-cloud-config-v1.json
var schemaText string

func validateCloudConfig(userData []byte) error {
	if !bytes.HasPrefix(userData, []byte("#cloud-config")) {
		return fmt.Errorf("missing #cloud-config")
	}
	var m interface{}
	err := yaml.Unmarshal(userData, &m)
	if err != nil {
		return err
	}
	compiler := jsonschema.NewCompiler()
	compiler.ExtractAnnotations = true
	if err := compiler.AddResource(schemaURL, strings.NewReader(schemaText)); err != nil {
		return err
	}
	schema, err := compiler.Compile(schemaURL)
	if err != nil {
		return err
	}
	if err := schema.Validate(m); err != nil {
		return err
	}
	return err
}
