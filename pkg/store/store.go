// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package store

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lima-vm/lima/v2/pkg/identifiers"
	"github.com/lima-vm/lima/v2/pkg/limatype"
	"github.com/lima-vm/lima/v2/pkg/limayaml"
	"github.com/lima-vm/lima/v2/pkg/store/dirnames"
	"github.com/lima-vm/lima/v2/pkg/store/filenames"
)

// Directory returns the LimaDir.
func Directory() string {
	limaDir, err := dirnames.LimaDir()
	if err != nil {
		return ""
	}
	return limaDir
}

// Validate checks the LimaDir.
func Validate() error {
	limaDir, err := dirnames.LimaDir()
	if err != nil {
		return err
	}
	names, err := Instances()
	if err != nil {
		return err
	}
	for _, name := range names {
		// Each instance directory needs to have limayaml
		instDir := filepath.Join(limaDir, name)
		yamlPath := filepath.Join(instDir, filenames.LimaYAML)
		if _, err := os.Stat(yamlPath); err != nil {
			return err
		}
	}
	return nil
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

// Instances returns the names of the instances under LimaDir.
func Instances() ([]string, error) {
	limaDir, err := dirnames.LimaDir()
	if err != nil {
		return nil, err
	}
	limaDirList, err := os.ReadDir(limaDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var names []string
	for _, f := range limaDirList {
		if strings.HasPrefix(f.Name(), ".") || strings.HasPrefix(f.Name(), "_") {
			continue
		}
		if !f.IsDir() {
			continue
		}
		names = append(names, f.Name())
	}
	return names, nil
}

func Disks() ([]string, error) {
	limaDiskDir, err := dirnames.LimaDisksDir()
	if err != nil {
		return nil, err
	}
	limaDiskDirList, err := os.ReadDir(limaDiskDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var names []string
	for _, f := range limaDiskDirList {
		names = append(names, f.Name())
	}
	return names, nil
}

// InstanceDir returns the instance dir.
// InstanceDir does not check whether the instance exists.
func InstanceDir(name string) (string, error) {
	if err := ValidateInstName(name); err != nil {
		return "", err
	}
	limaDir, err := dirnames.LimaDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(limaDir, name)
	return dir, nil
}

func DiskDir(name string) (string, error) {
	if err := identifiers.Validate(name); err != nil {
		return "", err
	}
	limaDisksDir, err := dirnames.LimaDisksDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(limaDisksDir, name)
	return dir, nil
}

// LoadYAMLByFilePath loads and validates the yaml.
func LoadYAMLByFilePath(ctx context.Context, filePath string) (*limatype.LimaYAML, error) {
	// We need to use the absolute path because it may be used to determine hostSocket locations.
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return nil, err
	}
	yContent, err := os.ReadFile(absPath)
	if err != nil {
		return nil, err
	}
	y, err := limayaml.Load(ctx, yContent, absPath)
	if err != nil {
		return nil, err
	}
	if err := limayaml.Validate(y, false); err != nil {
		return nil, err
	}
	return y, nil
}
