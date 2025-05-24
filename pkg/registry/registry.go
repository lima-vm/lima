// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package registry

import (
	"sync"

	"github.com/lima-vm/lima/pkg/driver"
)

type Registry struct {
	drivers map[string]driver.Driver
	mu      sync.RWMutex
}

func NewRegistry() *Registry {
	return &Registry{
		drivers: make(map[string]driver.Driver),
	}
}

func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var names []string
	for name := range r.drivers {
		names = append(names, name)
	}
	return names
}

func (r *Registry) Get(name string) (driver.Driver, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	driver, exists := r.drivers[name]
	return driver, exists
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
