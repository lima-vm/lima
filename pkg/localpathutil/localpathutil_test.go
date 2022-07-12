package localpathutil

import (
	"path/filepath"
	"testing"

	"gotest.tools/v3/assert"
)

func TestExpandTilde(t *testing.T) {
	_, err := Expand("~")
	assert.NilError(t, err)
}

func TestExpandDir(t *testing.T) {
	h, err := Expand("~")
	assert.NilError(t, err)
	d, err := Expand("~/foo")
	assert.NilError(t, err)
	// make sure this uses the local filepath
	assert.Equal(t, d, filepath.Join(h, "foo"))
}

func TestExpandHome(t *testing.T) {
	p, err := ExpandHome("~/bar", "/home/user")
	assert.NilError(t, err)
	// make sure this uses the unix path
	assert.Equal(t, p, "/home/user/bar")
}
