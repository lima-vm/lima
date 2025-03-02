/*
Copyright The Lima Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package store

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/containerd/containerd/identifiers"
	"github.com/lima-vm/lima/pkg/limayaml"
	"github.com/lima-vm/lima/pkg/store/dirnames"
	"github.com/lima-vm/lima/pkg/store/filenames"
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
	if err := identifiers.Validate(name); err != nil {
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
func LoadYAMLByFilePath(filePath string) (*limayaml.LimaYAML, error) {
	// We need to use the absolute path because it may be used to determine hostSocket locations.
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return nil, err
	}
	yContent, err := os.ReadFile(absPath)
	if err != nil {
		return nil, err
	}
	y, err := limayaml.Load(yContent, absPath)
	if err != nil {
		return nil, err
	}
	if err := limayaml.Validate(y, false); err != nil {
		return nil, err
	}
	return y, nil
}
