// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package driverutil

import (
	"github.com/lima-vm/lima/pkg/registry"
)

// AvailableDrivers returns a list of available driver names
func AvailableDrivers() []string {
	var available []string

	for _, name := range registry.DefaultRegistry.List() {
		driver, _ := registry.DefaultRegistry.Get(name)
		if err := driver.Validate(); err == nil {
			return available
		}
		available = append(available, name)
	}

	return available
}
