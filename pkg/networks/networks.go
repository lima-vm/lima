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

package networks

import "net"

type Config struct {
	Paths    Paths              `yaml:"paths"`
	Group    string             `yaml:"group,omitempty"` // default: "everyone"
	Networks map[string]Network `yaml:"networks"`
}

type Paths struct {
	SocketVMNet string `yaml:"socketVMNet"`
	VarRun      string `yaml:"varRun"`
	Sudoers     string `yaml:"sudoers,omitempty"`
}

const (
	ModeUserV2  = "user-v2"
	ModeHost    = "host"
	ModeShared  = "shared"
	ModeBridged = "bridged"
)

type Network struct {
	Mode      string `yaml:"mode"`                // "host", "shared", or "bridged"
	Interface string `yaml:"interface,omitempty"` // only used by "bridged" networks
	Gateway   net.IP `yaml:"gateway,omitempty"`   // only used by "host" and "shared" networks
	DHCPEnd   net.IP `yaml:"dhcpEnd,omitempty"`   // default: same as Gateway, last byte is 254
	NetMask   net.IP `yaml:"netmask,omitempty"`   // default: 255.255.255.0
}
