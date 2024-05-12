package limayaml

import (
	"os"
	"runtime"
	"testing"

	"gotest.tools/v3/assert"
)

func TestValidateEmpty(t *testing.T) {
	y, err := Load([]byte{}, "empty.yaml")
	assert.NilError(t, err)
	err = Validate(y, false)
	assert.Error(t, err, "field `images` must be set")
}

// Note: can't embed symbolic links, use "os"

func TestValidateDefault(t *testing.T) {
	if runtime.GOOS == "windows" {
		// FIXME: `assertion failed: error is not nil: field `mounts[1].location` must be an absolute path, got "/tmp/lima"`
		t.Skip("Skipping on windows")
	}

	bytes, err := os.ReadFile("default.yaml")
	assert.NilError(t, err)
	y, err := Load(bytes, "default.yaml")
	assert.NilError(t, err)
	err = Validate(y, true)
	assert.NilError(t, err)
}
