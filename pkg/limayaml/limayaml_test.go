package limayaml

import (
	"encoding/json"
	"os"
	"testing"

	"gotest.tools/v3/assert"
)

func dumpJSON(d interface{}) string {
	b, err := json.Marshal(d)
	if err != nil {
		return "ERROR"
	}
	return string(b)
}

const emptyYAML = "images: []\n"

func TestEmptyYAML(t *testing.T) {
	var y LimaYAML
	t.Log(dumpJSON(y))
	b, err := marshalYAML(y)
	assert.NilError(t, err)
	assert.Equal(t, string(b), emptyYAML)
}

const defaultYAML = `images: []
ssh:
  localPort: 0
`

func TestDefaultYAML(t *testing.T) {
	bytes, err := os.ReadFile("default.yaml")
	assert.NilError(t, err)
	var y LimaYAML
	err = unmarshalYAML(bytes, &y, "")
	assert.NilError(t, err)
	y.Images = nil // remove default images
	y.Mounts = nil // remove default mounts
	t.Log(dumpJSON(y))
	b, err := marshalYAML(y)
	assert.NilError(t, err)
	assert.Equal(t, string(b), defaultYAML)
}
