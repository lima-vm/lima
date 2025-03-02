/*
Copyright The Lima Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package driverutil

import (
	"github.com/lima-vm/lima/pkg/limayaml"
	"github.com/lima-vm/lima/pkg/vz"
	"github.com/lima-vm/lima/pkg/wsl2"
)

// Drivers returns the available drivers.
func Drivers() []string {
	drivers := []string{limayaml.QEMU}
	if vz.Enabled {
		drivers = append(drivers, limayaml.VZ)
	}
	if wsl2.Enabled {
		drivers = append(drivers, limayaml.WSL2)
	}
	return drivers
}
