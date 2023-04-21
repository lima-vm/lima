package networks

import (
	_ "embed"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"

	"github.com/goccy/go-yaml"
	"github.com/lima-vm/lima/pkg/store/dirnames"
	"github.com/lima-vm/lima/pkg/store/filenames"
	"github.com/lima-vm/lima/pkg/textutil"
	"github.com/sirupsen/logrus"
)

//go:embed networks.TEMPLATE.yaml
var defaultConfigTemplate string

type defaultConfigTemplateArgs struct {
	SocketVMNet string // "/opt/socket_vmnet/bin/socket_vmnet"
}

func defaultConfigBytes() ([]byte, error) {
	var args defaultConfigTemplateArgs
	candidates := []string{
		"/opt/socket_vmnet/bin/socket_vmnet", // the hard-coded path before v0.14
		"socket_vmnet",
		"/usr/local/opt/socket_vmnet/bin/socket_vmnet",    // Homebrew (Intel)
		"/opt/homebrew/opt/socket_vmnet/bin/socket_vmnet", // Homebrew (ARM)
	}
	for _, candidate := range candidates {
		if p, err := exec.LookPath(candidate); err == nil {
			realP, evalErr := filepath.EvalSymlinks(p)
			if evalErr != nil {
				return nil, evalErr
			}
			args.SocketVMNet = realP
			break
		} else if errors.Is(err, exec.ErrNotFound) || errors.Is(err, os.ErrNotExist) {
			logrus.WithError(err).Debugf("Failed to look up socket_vmnet path %q", candidate)
		} else {
			logrus.WithError(err).Warnf("Failed to look up socket_vmnet path %q", candidate)
		}
	}
	if args.SocketVMNet == "" {
		args.SocketVMNet = candidates[0] // the hard-coded path before v0.14
	}
	return textutil.ExecuteTemplate(defaultConfigTemplate, args)
}

func DefaultConfig() (YAML, error) {
	var config YAML
	defaultConfig, err := defaultConfigBytes()
	if err != nil {
		return config, err
	}
	err = yaml.UnmarshalWithOptions(defaultConfig, &config, yaml.Strict())
	if err != nil {
		return config, err
	}
	return config, nil
}

var cache struct {
	sync.Once
	config YAML
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
			var defaultConfig []byte
			defaultConfig, cache.err = defaultConfigBytes()
			if cache.err != nil {
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
		cache.err = yaml.UnmarshalWithOptions(b, &cache.config, yaml.Strict())
		if cache.err != nil {
			cache.err = fmt.Errorf("cannot parse %q: %w", configFile, cache.err)
		}
	})
}

// Config returns the network config from the _config/networks.yaml file.
func Config() (YAML, error) {
	loadCache()
	return cache.config, cache.err
}

// Sock returns a socket_vmnet socket.
func Sock(name string) (string, error) {
	loadCache()
	if cache.err != nil {
		return "", cache.err
	}
	if err := cache.config.Check(name); err != nil {
		return "", err
	}
	if cache.config.Paths.SocketVMNet == "" {
		return "", errors.New("socketVMNet is not set")
	}
	return cache.config.Sock(name), nil
}

// Usernet Returns true if the given network name is usernet network
func Usernet(name string) (bool, error) {
	loadCache()
	if cache.err != nil {
		return false, cache.err
	}
	return cache.config.Usernet(name)
}

// VDESock returns a vde socket.
//
// Deprecated. Use Sock.
func VDESock(name string) (string, error) {
	loadCache()
	if cache.err != nil {
		return "", cache.err
	}
	if err := cache.config.Check(name); err != nil {
		return "", err
	}
	if cache.config.Paths.VDEVMNet == "" {
		return "", errors.New("vdeVMnet is not set")
	}
	return cache.config.VDESock(name), nil
}
