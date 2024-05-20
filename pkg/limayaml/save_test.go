package limayaml

import (
	"testing"

	"github.com/lima-vm/lima/pkg/ptr"
	"gotest.tools/v3/assert"
)

func TestSaveEmpty(t *testing.T) {
	_, err := Save(&LimaYAML{})
	assert.NilError(t, err)
}

func TestSaveTilde(t *testing.T) {
	y := LimaYAML{
		Mounts: []Mount{
			{Location: "~", Writable: ptr.Of(false)},
			{Location: "/tmp/lima", Writable: ptr.Of(true)},
			{Location: "null"},
		},
	}
	b, err := Save(&y)
	assert.NilError(t, err)
	// yaml will load ~ (or null) as null
	// make sure that it is always quoted
	assert.Equal(t, string(b), `images: []
mounts:
- location: "~"
  writable: false
- location: /tmp/lima
  writable: true
- location: "null"
`)
}
