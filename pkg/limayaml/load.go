package limayaml

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/lima-vm/lima/pkg/store/dirnames"
	"github.com/lima-vm/lima/pkg/store/filenames"
	"github.com/sirupsen/logrus"
)

// Load loads the yaml and fulfills unspecified fields with the default values.
//
// Load does not validate. Use Validate for validation.
func Load(b []byte, filePath string) (*LimaYAML, error) {
	return load(b, filePath, false)
}

// LoadWithWarnings will call FillDefaults with warnings enabled (e.g. when
// the username is not valid on Linux and must be replaced by "Lima").
// It is called when creating or editing an instance.
func LoadWithWarnings(b []byte, filePath string) (*LimaYAML, error) {
	return load(b, filePath, true)
}

func load(b []byte, filePath string, warn bool) (*LimaYAML, error) {
	var y, d, o LimaYAML

	if err := Unmarshal(b, &y, fmt.Sprintf("main file %q", filePath)); err != nil {
		return nil, err
	}
	configDir, err := dirnames.LimaConfigDir()
	if err != nil {
		return nil, err
	}

	defaultPath := filepath.Join(configDir, filenames.Default)
	bytes, err := os.ReadFile(defaultPath)
	if err == nil {
		logrus.Debugf("Mixing %q into %q", defaultPath, filePath)
		if err := Unmarshal(bytes, &d, fmt.Sprintf("default file %q", defaultPath)); err != nil {
			return nil, err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}

	overridePath := filepath.Join(configDir, filenames.Override)
	bytes, err = os.ReadFile(overridePath)
	if err == nil {
		logrus.Debugf("Mixing %q into %q", overridePath, filePath)
		if err := Unmarshal(bytes, &o, fmt.Sprintf("override file %q", overridePath)); err != nil {
			return nil, err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}

	// It should be called before the `y` parameter is passed to FillDefault() that execute template.
	if err := ValidateParamIsUsed(&y); err != nil {
		return nil, err
	}

	FillDefault(&y, &d, &o, filePath, warn)
	return &y, nil
}
