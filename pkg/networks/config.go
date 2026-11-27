// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package networks

import (
	_ "embed"
	"errors"
	"fmt"
	"maps"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"sync"

	"github.com/goccy/go-yaml"
	"github.com/sirupsen/logrus"

	"github.com/lima-vm/lima/v2/pkg/limatype/dirnames"
	"github.com/lima-vm/lima/v2/pkg/limatype/filenames"
	"github.com/lima-vm/lima/v2/pkg/textutil"
	"github.com/lima-vm/lima/v2/pkg/usrlocal"
)

//go:embed networks.TEMPLATE.yaml
var defaultConfigTemplate string

type defaultConfigTemplateArgs struct {
	SocketVMNet      string // macOS: "/opt/socket_vmnet/bin/socket_vmnet"
	VarRun           string
	Sudoers          string
	Group            string
	BridgedInterface string
}

// socketVMNetPath returns the path of the installed socket_vmnet binary, or the
// hard-coded pre-v0.14 location when it is not installed.
func socketVMNetPath() (string, error) {
	candidates := []string{
		"/opt/socket_vmnet/bin/socket_vmnet", // the hard-coded path before v0.14
		"socket_vmnet",
		"/usr/local/opt/socket_vmnet/bin/socket_vmnet",    // Homebrew (Intel)
		"/opt/homebrew/opt/socket_vmnet/bin/socket_vmnet", // Homebrew (ARM)
	}
	for _, candidate := range candidates {
		if p, err := exec.LookPath(candidate); err == nil {
			return filepath.EvalSymlinks(p)
		} else if errors.Is(err, exec.ErrNotFound) || errors.Is(err, os.ErrNotExist) {
			logrus.WithError(err).Debugf("Failed to look up socket_vmnet path %#q", candidate)
		} else {
			logrus.WithError(err).Warnf("Failed to look up socket_vmnet path %#q", candidate)
		}
	}
	return candidates[0], nil
}

// defaultGroup returns the group that is allowed to run the network helper via
// sudo: "admin" on macOS, and the distribution's admin group on Linux.
func defaultGroup() string {
	if runtime.GOOS != "linux" {
		return "admin"
	}
	for _, name := range []string{"sudo", "wheel"} {
		if _, err := user.LookupGroup(name); err == nil {
			return name
		}
	}
	return "root"
}

func defaultConfigBytes() ([]byte, error) {
	args := defaultConfigTemplateArgs{
		VarRun:           "/private/var/run/lima",
		Sudoers:          "/private/etc/sudoers.d/lima",
		Group:            defaultGroup(),
		BridgedInterface: "en0",
	}
	switch runtime.GOOS {
	case "darwin":
		var err error
		if args.SocketVMNet, err = socketVMNetPath(); err != nil {
			return nil, err
		}
	case "linux":
		args.VarRun = "/run/lima"
		args.Sudoers = "/etc/sudoers.d/lima"
		// Lima never enslaves a physical interface, so this must be an existing bridge.
		args.BridgedInterface = "br0"
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
	return cfg, nil
}

// validateExternal validates the networks with `external: true`. loadCache applies it to
// every configuration before fillDefaults, so unlike Validate() it must not depend on the
// filesystem.
func (c *Config) validateExternal() error {
	for name, nw := range c.Networks {
		if !nw.External {
			continue
		}
		if nw.Mode != ModeUserV2 {
			return fmt.Errorf("network %#q: `external` is only supported for mode %#q, not %#q", name, ModeUserV2, nw.Mode)
		}
		// fillDefaults overwrites the "user-v2" network with the default one when no
		// user-v2 network has a gateway, which would silently drop `external`. The
		// guest is also configured from the gateway, so it has to match the daemon.
		if nw.Gateway == nil {
			return fmt.Errorf("network %#q: `external` requires `gateway` to be set to the gateway of the external daemon", name)
		}
		// Without a netmask, the subnet of the network would be computed as /0.
		if nw.NetMask == nil {
			return fmt.Errorf("network %#q: `external` requires `netmask` to be set to the netmask of the external daemon", name)
		}
	}
	return nil
}

// validateNetworkKeys returns an error if an entry under `networks` has an unknown field.
//
// Unknown fields elsewhere in networks.yaml are only a warning, for compatibility with fields that were
// removed from `paths`. Inside a network entry they are an error: a misspelled `external` would otherwise
// be ignored, and Lima would start its own daemon for a network that is meant to be served by another one.
func validateNetworkKeys(b []byte) error {
	var raw struct {
		Networks map[string]map[string]any `yaml:"networks"`
	}
	if err := yaml.Unmarshal(b, &raw); err != nil {
		return err
	}
	known := make(map[string]bool)
	for f := range reflect.TypeFor[Network]().Fields() {
		key, _, _ := strings.Cut(f.Tag.Get("yaml"), ",")
		known[key] = true
	}
	for _, name := range slices.Sorted(maps.Keys(raw.Networks)) {
		for _, key := range slices.Sorted(maps.Keys(raw.Networks[name])) {
			if !known[key] {
				return fmt.Errorf("network %#q: unknown field %#q", name, key)
			}
		}
	}
	return nil
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
				cache.err = fmt.Errorf("could not create %#q directory: %w", cfgDir, cache.err)
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
			cache.err = fmt.Errorf("cannot parse %#q: %w", cfgFile, cache.err)
			return
		}
		var strictCfg Config
		if strictErr := yaml.UnmarshalWithOptions(b, &strictCfg, yaml.Strict()); strictErr != nil {
			// Allow non-existing YAML fields, as a cfg created with Lima < v0.22 contains `vdeSwitch` and `vdeVMNet`.
			// These fields were removed in Lima v0.22.
			logrus.WithError(strictErr).Warn("Non-strict YAML is deprecated and will be unsupported in a future version of Lima: " + cfgFile)
		}
		if cache.err = validateNetworkKeys(b); cache.err != nil {
			cache.err = fmt.Errorf("invalid %#q: %w", cfgFile, cache.err)
			return
		}
		if cache.err = cache.cfg.validateExternal(); cache.err != nil {
			cache.err = fmt.Errorf("invalid %#q: %w", cfgFile, cache.err)
			return
		}
		cache.cfg, cache.err = fillDefaults(cache.cfg)
		if cache.err != nil {
			cache.err = fmt.Errorf("cannot fill default %#q: %w", cfgFile, cache.err)
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

func limaPrivilegedNetPath() (string, error) {
	dirs, err := usrlocal.LibexecLima()
	if err != nil {
		return "", err
	}
	if len(dirs) == 0 {
		return "", errors.New("could not find the libexec/lima directory")
	}
	path := filepath.Join(dirs[0], privilegedSubdir, LimaPrivilegedNet)
	if !filepath.IsAbs(path) {
		return "", fmt.Errorf("path %#q is not an absolute path", path)
	}
	if strings.ContainsAny(path, " \t\r\n\v\f") {
		return "", fmt.Errorf("path %#q contains whitespace", path)
	}
	if strings.ContainsAny(path, `,=:#!"\*?[]`) {
		return "", fmt.Errorf("path %#q contains sudoers metacharacters", path)
	}
	return path, nil
}
