// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package store

import (
	"testing"

	"gotest.tools/v3/assert"
)

func TestValidateInstName(t *testing.T) {
	instNames := []string{
		"default",
		"Ubuntu-20.04",
		"example.com",
		"under_score",
		"1-2_3.4",
		"yml",
		"yaml",
		"foo.yaml.com",
	}
	for _, arg := range instNames {
		t.Run(arg, func(t *testing.T) {
			err := ValidateInstName(arg)
			assert.NilError(t, err)
		})
	}
	invalidIdentifiers := []string{
		"",
		"my/instance",
		"my\\instance",
		"c:default",
		"dot.",
		".dot",
		"dot..dot",
		"underscore_",
		"_underscore",
		"underscore__underscore",
		"dash-",
		"-dash",
		"dash--dash",
	}
	for _, arg := range invalidIdentifiers {
		t.Run(arg, func(t *testing.T) {
			err := ValidateInstName(arg)
			assert.ErrorContains(t, err, "not a valid identifier")
		})
	}
	yamlNames := []string{
		"default.yaml",
		"MY.YAML",
		"My.YmL",
	}
	for _, arg := range yamlNames {
		t.Run(arg, func(t *testing.T) {
			err := ValidateInstName(arg)
			assert.ErrorContains(t, err, "must not end with .y")
		})
	}
}
