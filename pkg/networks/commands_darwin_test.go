package networks

import (
	"path/filepath"
	"testing"

	"gotest.tools/v3/assert"
)

func TestVDESock(t *testing.T) {
	config, err := DefaultConfig()
	assert.NilError(t, err)

	vdeSock := config.VDESock("foo")
	varRunDir := filepath.Join("/", "private", "var", "run", "lima")
	assert.Equal(t, vdeSock, filepath.Join(varRunDir, "foo.ctl"))
}

func TestPIDFile(t *testing.T) {
	config, err := DefaultConfig()
	assert.NilError(t, err)

	pidFile := config.PIDFile("name", "daemon")
	varRunDir := filepath.Join("/", "private", "var", "run", "lima")
	assert.Equal(t, pidFile, filepath.Join(varRunDir, "name_daemon.pid"))
}
