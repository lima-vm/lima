// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package jsonschemautil

import (
	"testing"

	"gotest.tools/v3/assert"
)

func TestValidateValidInstance(t *testing.T) {
	schema := "testdata/schema.json"
	instance := "testdata/valid.yaml"
	err := Validate(schema, instance)
	assert.NilError(t, err)
}

func TestValidateInvalidInstance(t *testing.T) {
	schema := "testdata/schema.json"
	instance := "testdata/invalid.yaml"
	err := Validate(schema, instance)
	assert.ErrorContains(t, err, "jsonschema validation failed")
}
