// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package networks

import (
	"net"
	"net/netip"
)

type Config struct {
	Paths    Paths                  `yaml:"paths" json:"paths"`
	Group    string                 `yaml:"group,omitempty" json:"group,omitempty"` // default: "everyone"
	Networks map[string]Network     `yaml:"networks" json:"networks"`
	Vmnet    map[string]VmnetConfig `yaml:"vmnet" json:"vmnet"`
}

type Paths struct {
	SocketVMNet string `yaml:"socketVMNet" json:"socketVMNet"`
	VarRun      string `yaml:"varRun" json:"varRun"`
	Sudoers     string `yaml:"sudoers,omitempty" json:"sudoers,omitempty"`
}

const (
	ModeUserV2  = "user-v2"
	ModeHost    = "host"
	ModeShared  = "shared"
	ModeBridged = "bridged"
)

var Modes = []string{
	ModeUserV2,
	ModeHost,
	ModeShared,
	ModeBridged,
}

type Network struct {
	Mode      string `yaml:"mode" json:"mode"`                               // "user-v2", "host", "shared", or "bridged"
	Interface string `yaml:"interface,omitempty" json:"interface,omitempty"` // only used by "bridged" networks
	Gateway   net.IP `yaml:"gateway,omitempty" json:"gateway,omitempty"`     // only used by "user-v2", "host" and "shared" networks
	DHCPEnd   net.IP `yaml:"dhcpEnd,omitempty" json:"dhcpEnd,omitempty"`     // default: same as Gateway, last byte is 254
	NetMask   net.IP `yaml:"netmask,omitempty" json:"netmask,omitempty"`     // default: 255.255.255.0
}

type VmnetMode string

const (
	VmnetModeShared VmnetMode = "shared"
	VmnetModeHost   VmnetMode = "host"
)

type VmnetConfig struct {
	Mode                VmnetMode    `yaml:"mode,omitempty" json:"mode,omitempty"` // "shared" or "host"
	Dhcp                *bool        `yaml:"dhcp,omitempty" json:"dhcp,omitempty"`
	DNSProxy            *bool        `yaml:"dnsProxy,omitempty" json:"dnsProxy,omitempty"`
	Mtu                 uint32       `yaml:"mtu,omitempty" json:"mtu,omitempty"`
	Nat44               *bool        `yaml:"nat44,omitempty" json:"nat44,omitempty"`
	Nat66               *bool        `yaml:"nat66,omitempty" json:"nat66,omitempty"`
	RouterAdvertisement *bool        `yaml:"routerAdvertisement,omitempty" json:"routerAdvertisement,omitempty"`
	Subnet              netip.Prefix `yaml:"subnet,omitempty" json:"subnet,omitempty"`
}
