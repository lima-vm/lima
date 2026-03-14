// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package jsonschemautil

import (
	"os"

	"github.com/goccy/go-yaml"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

func Validate(schemafile, instancefile string) error {
	compiler := jsonschema.NewCompiler()
	schema, err := compiler.Compile(schemafile)
	if err != nil {
		return err
	}
	instance, err := os.ReadFile(instancefile)
	if err != nil {
		return err
	}
	var y any
	err = yaml.Unmarshal(instance, &y)
	if err != nil {
		return err
	}
	return schema.Validate(y)
}
