package networks

import (
	_ "embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"

	"github.com/lima-vm/lima/pkg/store/dirnames"
	"github.com/lima-vm/lima/pkg/store/filenames"
	"gopkg.in/yaml.v2"
)

//go:embed networks.yaml
var defaultConfig []byte

func DefaultConfig() (NetworksConfig, error) {
	var config NetworksConfig
	err := yaml.Unmarshal(defaultConfig, &config)
	return config, err
}

var cache struct {
	sync.Once
	config NetworksConfig
	err    error
}

func ConfigFile() (string, error) {
	configDir, err := dirnames.LimaConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, filenames.NetworksConfig), nil
}

// loadCache loads the _config/networks.yaml file into the cache.
func loadCache() {
	cache.Do(func() {
		var configFile string
		configFile, cache.err = ConfigFile()
		if cache.err != nil {
			return
		}
		_, cache.err = os.Stat(configFile)
		if cache.err != nil {
			if !errors.Is(cache.err, os.ErrNotExist) {
				return
			}
			configDir := filepath.Dir(configFile)
			cache.err = os.MkdirAll(configDir, 0755)
			if cache.err != nil {
				cache.err = fmt.Errorf("could not create %q directory: %w", configDir, cache.err)
				return
			}
			cache.err = os.WriteFile(configFile, defaultConfig, 0644)
			if cache.err != nil {
				return
			}
		}
		var b []byte
		b, cache.err = os.ReadFile(configFile)
		if cache.err != nil {
			return
		}
		cache.err = yaml.Unmarshal(b, &cache.config)
		if cache.err != nil {
			cache.err = fmt.Errorf("cannot parse %q: %w", configFile, cache.err)
		}
	})
}

// Config returns the network config from the _config/networks.yaml file.
func Config() (NetworksConfig, error) {
	if runtime.GOOS != "darwin" {
		return NetworksConfig{}, errors.New("networks.yaml configuration is only supported on macOS right now")
	}
	loadCache()
	return cache.config, cache.err
}

func VDESock(name string) (string, error) {
	loadCache()
	if cache.err != nil {
		return "", cache.err
	}
	if err := cache.config.Check(name); err != nil {
		return "", err
	}
	return cache.config.VDESock(name), nil
}
