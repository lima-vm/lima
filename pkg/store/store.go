package store

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/AkihiroSuda/lima/pkg/limayaml"
	"github.com/containerd/containerd/identifiers"
)

// DotLima is a directory that appears under the home directory.
const DotLima = ".lima"

// LimaDir returns the abstract path of `~/.lima`.
//
// NOTE: We do not use `~/Library/Application Support/Lima` on macOS.
// We use `~/.lima` so that we can have enough space for the length of the socket path,
// which can be only 104 characters on macOS.
func LimaDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(homeDir, DotLima)
	return dir, nil
}

// Instances returns the names of the instances under LimaDir.
func Instances() ([]string, error) {
	limaDir, err := LimaDir()
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
	limaDir, err := LimaDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(limaDir, name)
	return dir, nil
}

// LoadYAMLByFilePath loads and validates the yaml.
func LoadYAMLByFilePath(filePath string) (*limayaml.LimaYAML, error) {
	if _, err := os.Stat(filePath); err != nil {
		return nil, err
	}
	yContent, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	y, err := limayaml.Load(yContent)
	if err != nil {
		return nil, err
	}
	if err := limayaml.Validate(*y); err != nil {
		return nil, err
	}
	return y, nil
}
