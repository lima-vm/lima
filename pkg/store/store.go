package store

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/containerd/containerd/identifiers"
	"github.com/lima-vm/lima/pkg/limayaml"
	"github.com/lima-vm/lima/pkg/store/dirnames"
)

// Instances returns the names of the instances under LimaDir.
func Instances() ([]string, error) {
	limaDir, err := dirnames.LimaDir()
	if err != nil {
		return nil, err
	}
	limaDirList, err := os.ReadDir(limaDir)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, f := range limaDirList {
		if strings.HasPrefix(f.Name(), ".") || strings.HasPrefix(f.Name(), "_") {
			continue
		}
		names = append(names, f.Name())
	}
	return names, nil
}

// InstanceDir returns the instance dir.
// InstanceDir does not check whether the instance exists
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

// LoadYAML loads the yaml.
func LoadYAML(b []byte) (*limayaml.LimaYAML, error) {
	y, err := limayaml.Load(b, "")
	if err != nil {
		return nil, err
	}
	return y, nil
}

// LoadYAMLByFilePath loads and validates the yaml.
func LoadYAMLByFilePath(filePath string) (*limayaml.LimaYAML, error) {
	yContent, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	y, err := limayaml.Load(yContent, filePath)
	if err != nil {
		return nil, err
	}
	if err := limayaml.Validate(*y, false); err != nil {
		return nil, err
	}
	return y, nil
}

// SaveYAML saves the yaml.
func SaveYAML(y *limayaml.LimaYAML) ([]byte, error) {
	b, err := limayaml.Save(y)
	if err != nil {
		return nil, err
	}
	return b, nil
}

// SaveYAMLByFilePath validates and saves the yaml.
func SaveYAMLByFilePath(y *limayaml.LimaYAML, filePath string) ([]byte, error) {
	if err := limayaml.Validate(*y, false); err != nil {
		return nil, err
	}
	b, err := limayaml.Save(y)
	if err != nil {
		return nil, err
	}
	err = os.WriteFile(filePath, b, 0644)
	if err != nil {
		return nil, err
	}
	return b, nil
}
