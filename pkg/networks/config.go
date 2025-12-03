// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package networks

import (
	_ "embed"
	"errors"
	"fmt"
	"net/netip"
	"os"
	"os/exec"
	"path/filepath"
	"sync"

	"github.com/goccy/go-yaml"
	"github.com/sirupsen/logrus"

	"github.com/lima-vm/lima/v2/pkg/limatype/dirnames"
	"github.com/lima-vm/lima/v2/pkg/limatype/filenames"
	"github.com/lima-vm/lima/v2/pkg/ptr"
	"github.com/lima-vm/lima/v2/pkg/textutil"
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

func fillDefaults(cfg Config) (Config, error) {
	usernetFound := false
	if cfg.Networks == nil {
		cfg.Networks = make(map[string]Network)
	}
	for nw := range cfg.Networks {
		if cfg.Networks[nw].Mode == ModeUserV2 && cfg.Networks[nw].Gateway != nil {
			usernetFound = true
		}
	}
	if !usernetFound {
		defaultCfg, err := DefaultConfig()
		if err != nil {
			return cfg, err
		}
		cfg.Networks[ModeUserV2] = defaultCfg.Networks[ModeUserV2]
	}
	if len(cfg.Vmnet) == 0 {
		defaultCfg, err := DefaultConfig()
		if err != nil {
			return cfg, err
		}
		cfg.Vmnet = defaultCfg.Vmnet
	}
	// Fill default values for VmnetConfig
	for id, vmnetCfg := range cfg.Vmnet {
		if vmnetCfg.Mode == "" {
			vmnetCfg.Mode = VmnetModeShared
		}
		if vmnetCfg.Dhcp == nil {
			vmnetCfg.Dhcp = ptr.Of(true)
		}
		if vmnetCfg.DNSProxy == nil {
			vmnetCfg.DNSProxy = ptr.Of(true)
		}
		if vmnetCfg.Mtu == 0 {
			vmnetCfg.Mtu = 1500
		}
		if vmnetCfg.Nat44 == nil {
			vmnetCfg.Nat44 = ptr.Of(true)
		}
		if vmnetCfg.Nat66 == nil {
			vmnetCfg.Nat66 = ptr.Of(true)
		}
		if vmnetCfg.RouterAdvertisement == nil {
			vmnetCfg.RouterAdvertisement = ptr.Of(true)
		}
		if vmnetCfg.Subnet.IsValid() {
			vmnetCfg.Subnet = netip.PrefixFrom(
				// VmnetConfig.SetIPv4Subnet ensures that the first address is the network address.
				vmnetCfg.Subnet.Masked().Addr().Next(),
				vmnetCfg.Subnet.Bits(),
			)
		}
		cfg.Vmnet[id] = vmnetCfg
	}
	return cfg, nil
}

func DefaultConfig() (Config, error) {
	var cfg Config
	b, err := defaultConfigBytes()
	if err != nil {
		return cfg, err
	}
	err = yaml.UnmarshalWithOptions(b, &cfg, yaml.Strict())
	if err != nil {
		return cfg, err
	}
	return cfg, nil
}

var cache struct {
	sync.Once
	cfg Config
	err error
}

func ConfigFile() (string, error) {
	cfgDir, err := dirnames.LimaConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cfgDir, filenames.NetworksConfig), nil
}

// loadCache loads the _config/networks.yaml file into the cache.
func loadCache() {
	cache.Do(func() {
		var cfgFile string
		cfgFile, cache.err = ConfigFile()
		if cache.err != nil {
			return
		}
		_, cache.err = os.Stat(cfgFile)
		if cache.err != nil {
			if !errors.Is(cache.err, os.ErrNotExist) {
				return
			}
			cfgDir := filepath.Dir(cfgFile)
			cache.err = os.MkdirAll(cfgDir, 0o755)
			if cache.err != nil {
				cache.err = fmt.Errorf("could not create %q directory: %w", cfgDir, cache.err)
				return
			}
			var b []byte
			b, cache.err = defaultConfigBytes()
			if cache.err != nil {
				return
			}
			cache.err = os.WriteFile(cfgFile, b, 0o644)
			if cache.err != nil {
				return
			}
		}
		var b []byte
		b, cache.err = os.ReadFile(cfgFile)
		if cache.err != nil {
			return
		}
		cache.err = yaml.Unmarshal(b, &cache.cfg)
		if cache.err != nil {
			cache.err = fmt.Errorf("cannot parse %q: %w", cfgFile, cache.err)
			return
		}
		var strictCfg Config
		if strictErr := yaml.UnmarshalWithOptions(b, &strictCfg, yaml.Strict()); strictErr != nil {
			// Allow non-existing YAML fields, as a cfg created with Lima < v0.22 contains `vdeSwitch` and `vdeVMNet`.
			// These fields were removed in Lima v0.22.
			logrus.WithError(strictErr).Warn("Non-strict YAML is deprecated and will be unsupported in a future version of Lima: " + cfgFile)
		}
		cache.cfg, cache.err = fillDefaults(cache.cfg)
		if cache.err != nil {
			cache.err = fmt.Errorf("cannot fill default %q: %w", cfgFile, cache.err)
		}
	})
}

// LoadConfig returns the network cfg from the _config/networks.yaml file.
func LoadConfig() (Config, error) {
	loadCache()
	return cache.cfg, cache.err
}

// Sock returns a socket_vmnet socket.
func Sock(name string) (string, error) {
	loadCache()
	if cache.err != nil {
		return "", cache.err
	}
	if err := cache.cfg.Check(name); err != nil {
		return "", err
	}
	if cache.cfg.Paths.SocketVMNet == "" {
		return "", errors.New("socketVMNet is not set")
	}
	return cache.cfg.Sock(name), nil
}

// IsUsernet returns true if the given network name is a usernet network.
// It return false if the cache cannot be loaded or the network is not defined.
func IsUsernet(name string) bool {
	loadCache()
	if cache.err != nil {
		return false
	}
	isUsernet, err := cache.cfg.Usernet(name)
	if err != nil {
		return false
	}
	return isUsernet
}

// Identifiers returns all network identifiers defined in the networks config.
func Identifiers() ([]string, error) {
	loadCache()
	if cache.err != nil {
		return nil, cache.err
	}
	ids := make([]string, 0, len(cache.cfg.Networks)+len(cache.cfg.Vmnet))
	for id := range cache.cfg.Networks {
		ids = append(ids, "lima:"+id)
	}
	for id := range cache.cfg.Vmnet {
		ids = append(ids, "vmnet:"+id)
	}
	return ids, nil
}
