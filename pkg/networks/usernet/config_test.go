package usernet

import (
	"path/filepath"
	"testing"

	"github.com/lima-vm/lima/pkg/store/dirnames"
	"github.com/lima-vm/lima/pkg/store/filenames"
	"gotest.tools/v3/assert"
)

func TestConfig(t *testing.T) {
	t.Run("max network path in instance directory", func(t *testing.T) {
		limaHome, err := dirnames.LimaDir()
		assert.NilError(t, err)
		longestInstDir := filepath.Join(limaHome, filenames.LongestSock)
		_, err = SockWithDirectory(longestInstDir, filenames.LongestSock, QEMUSock)
		assert.NilError(t, err)
	})

	t.Run("max network path in _networks directory", func(t *testing.T) {
		instName := filenames.LongestSock
		_, err := Sock(instName, EndpointSock)
		assert.NilError(t, err)
	})

	t.Run("throw error for invalid socket path", func(t *testing.T) {
		limaHome, err := dirnames.LimaDir()
		assert.NilError(t, err)
		longestInstDir := filepath.Join(limaHome, filenames.LongestSock)
		invalidName := filenames.LongestSock + filenames.LongestSock + filenames.LongestSock + filenames.LongestSock
		_, err = SockWithDirectory(longestInstDir, invalidName, QEMUSock)
		assert.ErrorContains(t, err, "must be less than UNIX_PATH_MAX=")
	})

	t.Run("default as name when empty", func(t *testing.T) {
		path, err := SockWithDirectory("/", "", QEMUSock)
		assert.NilError(t, err)
		assert.Equal(t, path, "/default_qemu.sock")
	})
}
