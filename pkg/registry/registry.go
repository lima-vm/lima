// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package registry

import (
	"sync"

	"github.com/lima-vm/lima/pkg/driver"
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
	// TODO: Implement plugin discovery logic
	return nil
}

var DefaultRegistry *Registry

func init() {
	DefaultRegistry = NewRegistry()
}

func Register(driver driver.Driver) {
	if DefaultRegistry != nil {
		name := driver.Name()
		if _, exists := DefaultRegistry.drivers[name]; exists {
			return
		}

		DefaultRegistry.drivers[name] = driver
	}
}
