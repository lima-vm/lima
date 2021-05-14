package store

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/AkihiroSuda/lima/pkg/limayaml"
	"github.com/containerd/containerd/identifiers"
)

const (
	// DotLima is a directory that appears under the home directory.
	DotLima = ".lima"
	// YAMLFileName appears under an instance dir.
	YAMLFileName = "lima.yaml"
)

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

// InstanceNameFromInstanceDir extracts the instance name
// from the path of the instance directory.
// e.g. "foo" for "/Users/somebody/.lima/foo".
func InstanceNameFromInstanceDir(s string) (string, error) {
	base := filepath.Base(s)
	return base, identifiers.Validate(base)
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

// YAMLFilePathByInstanceName returns the yaml file path but does not
// check whether it exists.
func YAMLFilePathByInstanceName(instName string) (string, string, error) {
	instDir, err := InstanceDir(instName)
	if err != nil {
		return "", instDir, err
	}
	instYAMLPath := filepath.Join(instDir, YAMLFileName)
	return instYAMLPath, instDir, nil
}

// LoadYAMLByInstanceName loads and validates the yaml.
// LoadYAMLByInstanceName may return os.ErrNotExist
func LoadYAMLByInstanceName(instName string) (*limayaml.LimaYAML, string, error) {
	instYAMLPath, instDir, err := YAMLFilePathByInstanceName(instName)
	if err != nil {
		return nil, "", err
	}
	y, err := LoadYAMLByFilePath(instYAMLPath)
	if err != nil {
		return nil, "", err
	}
	return y, instDir, nil
}
