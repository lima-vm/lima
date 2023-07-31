package usernet

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/lima-vm/lima/pkg/store/dirnames"
	"github.com/lima-vm/lima/pkg/store/filenames"
	"gotest.tools/v3/assert"
)

func TestConfig(t *testing.T) {
	t.Run("max network path in instance directory", func(t *testing.T) {
		limaHome, err := dirnames.LimaDir()
		assert.NilError(t, err)
		longName := strings.Repeat("a", len(filenames.LongestSock))
		longestInstDir := filepath.Join(limaHome, longName)
		_, err = SockWithDirectory(longestInstDir, longName, QEMUSock)
		assert.NilError(t, err)
	})

	t.Run("max network path in _networks directory", func(t *testing.T) {
		instName := strings.Repeat("a", len(filenames.LongestSock))
		_, err := Sock(instName, EndpointSock)
		assert.NilError(t, err)
	})

	t.Run("throw error for invalid socket path", func(t *testing.T) {
		limaHome, err := dirnames.LimaDir()
		assert.NilError(t, err)
		longName := strings.Repeat("a", len(filenames.LongestSock))
		longestInstDir := filepath.Join(limaHome, longName)
		invalidName := strings.Repeat(longName, 4)
		_, err = SockWithDirectory(longestInstDir, invalidName, QEMUSock)
		assert.ErrorContains(t, err, "must be less than UNIX_PATH_MAX=")
	})

	t.Run("default as name when empty", func(t *testing.T) {
		path, err := SockWithDirectory("test", "", QEMUSock)
		assert.NilError(t, err)
		assert.Equal(t, path, filepath.Join("test", "default_qemu.sock"))
	})
}
