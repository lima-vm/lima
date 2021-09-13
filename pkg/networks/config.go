package networks

import (
	_ "embed"
	"errors"
	"fmt"
	"gopkg.in/yaml.v2"
	"os"
	"path/filepath"
	"runtime"
	"sync"

	"github.com/lima-vm/lima/pkg/store/dirnames"
	"github.com/lima-vm/lima/pkg/store/filenames"
)

//go:embed networks.yaml
var defaultConfig []byte

var cache struct {
	sync.Once
	config NetworksConfig
	err    error
}

// load() loads the _config/networks.yaml file.
func load() {
	cache.Do(func() {
		var configDir string
		configDir, cache.err = dirnames.LimaConfigDir()
		if cache.err != nil {
			return
		}
		configFile := filepath.Join(configDir, filenames.NetworksConfig)
		_, cache.err = os.Stat(configFile)
		if cache.err != nil {
			if !errors.Is(cache.err, os.ErrNotExist) {
				return
			}
			cache.err = os.MkdirAll(configDir, 0700)
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
	if runtime.GOOS == "darwin" {
		load()
		return cache.config, cache.err
	}
	return NetworksConfig{}, errors.New("networks.yaml configuration is only supported on macOS right now")
}

func VDESock(name string) (string, error) {
	load()
	if cache.err != nil {
		return "", cache.err
	}
	if err := cache.config.Check(name); err != nil {
		return "", err
	}
	return cache.config.VDESock(name), nil
}
