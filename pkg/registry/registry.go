// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package registry

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/lima-vm/lima/pkg/driver"
	"github.com/lima-vm/lima/pkg/usrlocalsharelima"
	"github.com/sirupsen/logrus"
)

type Registry struct {
	drivers         map[string]driver.Driver
	externalDrivers map[string]string // For now mapping external driver names to paths
	mu              sync.RWMutex
}

func NewRegistry() *Registry {
	return &Registry{
		drivers:         make(map[string]driver.Driver),
		externalDrivers: make(map[string]string),
	}
}

func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var names []string
	for name := range r.drivers {
		names = append(names, name)
	}

	for name := range r.externalDrivers {
		names = append(names, name+" (external)")
	}
	return names
}

func (r *Registry) Get(name string) (driver.Driver, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	driver, exists := r.drivers[name]
	return driver, exists
}

func (r *Registry) GetExternalDriver(name string) (string, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	plugin, exists := r.externalDrivers[name]
	return plugin, exists
}

func (r *Registry) RegisterPlugin(name, path string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.externalDrivers[name]; exists {
		logrus.Debugf("Plugin %q is already registered, skipping", name)
		return
	}

	r.externalDrivers[name] = path
	logrus.Debugf("Registered plugin %q at %s", name, path)
}

func (r *Registry) DiscoverPlugins() error {
	limaShareDir, err := usrlocalsharelima.Dir()
	if err != nil {
		return fmt.Errorf("failed to determine Lima share directory: %w", err)
	}
	stdPluginDir := filepath.Join(filepath.Dir(limaShareDir), "libexec", "lima", "drivers")

	if _, err := os.Stat(stdPluginDir); err == nil {
		if err := r.discoverPluginsInDir(stdPluginDir); err != nil {
			logrus.Warnf("Error discovering plugins in %s: %v", stdPluginDir, err)
		}
	}

	if pluginPaths := os.Getenv("LIMA_DRIVERS_PATH"); pluginPaths != "" {
		paths := filepath.SplitList(pluginPaths)
		for _, path := range paths {
			if path == "" {
				continue
			}

			info, err := os.Stat(path)
			if err != nil {
				logrus.Warnf("Error accessing plugin path %s: %v", path, err)
				continue
			}

			if info.IsDir() {
				if err := r.discoverPluginsInDir(path); err != nil {
					logrus.Warnf("Error discovering plugins in %s: %v", path, err)
				}
			} else if isExecutable(info.Mode()) {
				r.registerPluginFile(path)
			}
		}
	}

	return nil
}

func (r *Registry) discoverPluginsInDir(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("failed to read plugin directory %s: %w", dir, err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			logrus.Warnf("Failed to get info for %s: %v", entry.Name(), err)
			continue
		}

		if !isExecutable(info.Mode()) {
			continue
		}

		pluginPath := filepath.Join(dir, entry.Name())
		r.registerPluginFile(pluginPath)
	}

	return nil
}

func (r *Registry) registerPluginFile(path string) {
	base := filepath.Base(path)
	if !strings.HasPrefix(base, "lima-plugin-") {
		return
	}

	name := strings.TrimPrefix(base, "lima-plugin-")
	name = strings.TrimSuffix(name, filepath.Ext(name))

	cmd := exec.Command(path, "--version")
	if err := cmd.Run(); err != nil {
		logrus.Warnf("Plugin %s failed version check: %v", path, err)
		return
	}

	r.RegisterPlugin(name, path)
}

func isExecutable(mode os.FileMode) bool {
	return mode&0111 != 0
}

var DefaultRegistry *Registry

func init() {
	DefaultRegistry = NewRegistry()
}

func Register(driver driver.Driver) {
	if DefaultRegistry != nil {
		name := driver.GetInfo().DriverName
		if _, exists := DefaultRegistry.drivers[name]; exists {
			return
		}

		DefaultRegistry.drivers[name] = driver
	}
}
