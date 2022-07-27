package dirnames

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/lima-vm/lima/pkg/store/filenames"
)

// DotLima is a directory that appears under the home directory.
const DotLima = ".lima"

// LimaDir returns the abstract path of `~/.lima` (or $LIMA_HOME, if set).
//
// NOTE: We do not use `~/Library/Application Support/Lima` on macOS.
// We use `~/.lima` so that we can have enough space for the length of the socket path,
// which can be only 104 characters on macOS.
//
// NOTE: There are some issues when using "long names" on Windows.
// We use "short names" here, so that it works with user names containing unicode etc.
// They normally have 8+3 characters, with suffix.
func LimaDir() (string, error) {
	dir := os.Getenv("LIMA_HOME")
	if dir == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		// on windows, 8.3 paths are needed by some tools like QEMU
		if runtime.GOOS == "windows" {
			homeDir, err = ShortPathName(homeDir)
			if err != nil {
				return "", err
			}
		}
		dir = filepath.Join(homeDir, DotLima)
	}
	if _, err := os.Stat(dir); errors.Is(err, os.ErrNotExist) {
		return dir, nil
	}
	// on windows, EvalSymlinks translates short paths back to long again
	if runtime.GOOS == "windows" {
		return dir, nil
	}
	realdir, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return "", fmt.Errorf("cannot evaluate symlinks in %q: %w", dir, err)
	}
	return realdir, nil
}

// LimaConfigDir returns the path of the config directory, $LIMA_HOME/_config.
func LimaConfigDir() (string, error) {
	limaDir, err := LimaDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(limaDir, filenames.ConfigDir), nil
}

// LimaNetworksDir returns the path of the networks log directory, $LIMA_HOME/_networks.
func LimaNetworksDir() (string, error) {
	limaDir, err := LimaDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(limaDir, filenames.NetworksDir), nil
}
