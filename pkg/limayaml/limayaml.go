package limayaml

import "github.com/opencontainers/go-digest"

type LimaYAML struct {
	Arch       Arch        `yaml:"arch,omitempty"`
	Images     []File      `yaml:"images"` // REQUIRED
	CPUs       int         `yaml:"cpus,omitempty"`
	Memory     string      `yaml:"memory,omitempty"` // go-units.RAMInBytes
	Disk       string      `yaml:"disk,omitempty"`   // go-units.RAMInBytes
	Mounts     []Mount     `yaml:"mounts,omitempty"`
	SSH        SSH         `yaml:"ssh,omitempty"` // REQUIRED (FIXME)
	Firmware   Firmware    `yaml:"firmware,omitempty"`
	Video      Video       `yaml:"video,omitempty"`
	Provision  []Provision `yaml:"provision,omitempty"`
	Containerd Containerd  `yaml:"containerd,omitempty"`
	Probes     []Probe     `yaml:"probes,omitempty"`
}

type Arch = string

const (
	X8664   Arch = "x86_64"
	AARCH64 Arch = "aarch64"
)

type File struct {
	Location string        `yaml:"location"` // REQUIRED
	Arch     Arch          `yaml:"arch,omitempty"`
	Digest   digest.Digest `yaml:"digest,omitempty"`
}

type Mount struct {
	Location string `yaml:"location"` // REQUIRED
	Writable bool   `yaml:"writable,omitempty"`
}

type SSH struct {
	LocalPort int `yaml:"localPort,omitempty"` // REQUIRED (FIXME: auto assign)

	// LoadDotSSHPubKeys loads ~/.ssh/*.pub in addition to $LIMA_HOME/_config/user.pub .
	// Default: true
	LoadDotSSHPubKeys *bool `yaml:"loadDotSSHPubKeys,omitempty"`
}

type Firmware struct {
	// LegacyBIOS disables UEFI if set.
	// LegacyBIOS is ignored for aarch64.
	LegacyBIOS bool `yaml:"legacyBIOS,omitempty"`
}

type Video struct {
	// Display is a QEMU display string
	Display string `yaml:"display,omitempty"`
}

type ProvisionMode = string

const (
	ProvisionModeSystem ProvisionMode = "system"
	ProvisionModeUser   ProvisionMode = "user"
)

type Provision struct {
	Mode   ProvisionMode `yaml:"mode"` // default: "system"
	Script string        `yaml:"script"`
}

type Containerd struct {
	System *bool `yaml:"system,omitempty"` // default: false
	User   *bool `yaml:"user,omitempty"`   // default: true
}

type ProbeMode = string

const (
	ProbeModeReadiness ProbeMode = "readiness"
)

type Probe struct {
	Mode        ProbeMode // default: "readiness"
	Description string
	Script      string
	Hint        string
}
