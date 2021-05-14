package limayaml

import (
	"testing"

	"gotest.tools/v3/assert"
)

func TestDefaultTemplateYAML(t *testing.T) {
	_, err := Load(DefaultTemplate)
	assert.NilError(t, err)
	// Do not call Validate(y) here, as it fails when `~/lima` is missing
}
