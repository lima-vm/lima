package dirnames

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/lima-vm/lima/pkg/store/filenames"
)

// DotLima is a directory that appears under the home directory.
const DotLima = ".lima"

// LimaDir returns the abstract path of `~/.lima` (or $LIMA_HOME, if set).
//
// NOTE: We do not use `~/Library/Application Support/Lima` on macOS.
// We use `~/.lima` so that we can have enough space for the length of the socket path,
// which can be only 104 characters on macOS.
func LimaDir() (string, error) {
	dir := os.Getenv("LIMA_HOME")
	if dir == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		dir = filepath.Join(homeDir, DotLima)
	}
	if _, err := os.Stat(dir); errors.Is(err, os.ErrNotExist) {
		return dir, nil
	}
	realdir, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return "", fmt.Errorf("cannot evaluate symlinks in %q: %w", dir, err)
	}
	return realdir, nil
}

// Lima is a directory that appears under the cache directory.
const Lima = "lima"

// LimaCacheDir returns the path of the cache directory (or $LIMA_CACHE, if set).
//
// NOTE: This uses ~/Library/Caches/lima on macOS, or $XDG_CACHE_HOME/lima on Linux.
func LimaCacheDir() (string, error) {
	dir := os.Getenv("LIMA_CACHE")
	if dir == "" {
		ucd, err := os.UserCacheDir()
		if err != nil {
			return "", err
		}
		dir = filepath.Join(ucd, Lima)
	}
	return dir, nil
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

// LimaDisksDir returns the path of the disks directory, $LIMA_HOME/_disks.
func LimaDisksDir() (string, error) {
	limaDir, err := LimaDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(limaDir, filenames.DisksDir), nil
}
