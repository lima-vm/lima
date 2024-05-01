package networks

import (
	"testing"

	"gotest.tools/v3/assert"
)

func TestSock(t *testing.T) {
	config, err := DefaultConfig()
	assert.NilError(t, err)

	sock := config.Sock("foo")
	assert.Equal(t, sock, "/private/var/run/lima/socket_vmnet.foo")
}

func TestPIDFile(t *testing.T) {
	config, err := DefaultConfig()
	assert.NilError(t, err)

	pidFile := config.PIDFile("name", "daemon")
	assert.Equal(t, pidFile, "/private/var/run/lima/name_daemon.pid")
}
