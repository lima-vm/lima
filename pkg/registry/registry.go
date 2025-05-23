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

func (r *Registry) Register(driver driver.Driver) {
	r.mu.Lock()
	defer r.mu.Unlock()

	name := driver.Name()
	if _, exists := r.drivers[name]; exists {
		return
	}

	r.drivers[name] = driver
}

func (r *Registry) Get(name string) (driver.Driver, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	driver, exists := r.drivers[name]
	return driver, exists
}

var DefaultRegistry *Registry

func Register(driver driver.Driver) {
	if DefaultRegistry != nil {
		DefaultRegistry.Register(driver)
	}
}
