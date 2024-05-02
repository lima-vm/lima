package cidata

import (
	"testing"

	"gotest.tools/v3/assert"
)

func TestValidate(t *testing.T) {
	config := `#cloud-config
users:
   - default
`
	err := ValidateCloudConfig([]byte(config))
	assert.NilError(t, err)
}
