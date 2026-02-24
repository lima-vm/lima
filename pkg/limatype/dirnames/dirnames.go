// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package dirnames

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lima-vm/lima/v2/pkg/identifiers"
	"github.com/lima-vm/lima/v2/pkg/limatype/filenames"
)

// DotLima is a directory that appears under the home directory.
const DotLima = ".lima"

// LimaDir returns the absolute path of `~/.lima` (or $LIMA_HOME, if set).
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

// LimaTemplatesDir returns the path of the templates directory, $LIMA_HOME/_templates.
func LimaTemplatesDir() (string, error) {
	limaDir, err := LimaDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(limaDir, filenames.TemplatesDir), nil
}

// LimaMntDir returns the path of the mount points directory, $LIMA_HOME/_mnt.
func LimaMntDir() (string, error) {
	limaDir, err := LimaDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(limaDir, filenames.MntDir), nil
}

// InstanceDir returns the instance dir.
// InstanceDir does not check whether the instance exists.
func InstanceDir(name string) (string, error) {
	if err := ValidateInstName(name); err != nil {
		return "", err
	}
	limaDir, err := LimaDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(limaDir, name)
	return dir, nil
}

// ValidateInstName checks if the name is a valid instance name. For this it needs to
// be a valid identifier, and not end in .yml or .yaml (case insensitively).
func ValidateInstName(name string) error {
	if err := identifiers.Validate(name); err != nil {
		return fmt.Errorf("instance name %q is not a valid identifier: %w", name, err)
	}
	lower := strings.ToLower(name)
	if strings.HasSuffix(lower, ".yml") || strings.HasSuffix(lower, ".yaml") {
		return fmt.Errorf("instance name %q must not end with .yml or .yaml suffix", name)
	}
	return nil
}
