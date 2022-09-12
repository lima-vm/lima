package networks

import "net"

type NetworksConfig struct {
	Paths    Paths              `yaml:"paths"`
	Group    string             `yaml:"group,omitempty"` // default: "everyone"
	Networks map[string]Network `yaml:"networks"`
}

type Paths struct {
	SocketVMNet string `yaml:"socketVMNet"`
	VDESwitch   string `yaml:"vdeSwitch"` // Deprecated
	VDEVMNet    string `yaml:"vdeVMNet"`  // Deprecated
	VarRun      string `yaml:"varRun"`
	Sudoers     string `yaml:"sudoers,omitempty"`
}

const (
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
