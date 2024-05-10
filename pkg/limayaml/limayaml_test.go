package limayaml

import (
	"os"
	"testing"

	"gotest.tools/v3/assert"
)

func TestDefaultYAML(t *testing.T) {
	bytes, err := os.ReadFile("default.yaml")
	assert.NilError(t, err)
	var y LimaYAML
	err = unmarshalYAML(bytes, &y, "")
	assert.NilError(t, err)
}
