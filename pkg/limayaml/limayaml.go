package limayaml

type LimaYAML struct {
	Arch     Arch     `yaml:"arch,omitempty"`
	Images   []Image  `yaml:"images"` // REQUIRED
	CPUs     int      `yaml:"cpus,omitempty"`
	Memory   string   `yaml:"memory,omitempty"` // go-units.RAMInBytes
	Disk     string   `yaml:"disk,omitempty"`   // go-units.RAMInBytes
	Mounts   []Mount  `yaml:"mounts,omitempty"`
	SSH      SSH      `yaml:"ssh,omitempty"` // REQUIRED (FIXME)
	Firmware Firmware `yaml:"firmware,omitempty"`
}

type Arch = string

const (
	X8664   Arch = "x86_64"
	AARCH64 Arch = "aarch64"
)

type Image struct {
	Location string `yaml:"location"` // REQUIRED
	Arch     string `yaml:"arch,omitempty"`
}

type Mount struct {
	Location string `yaml:"location"` // REQUIRED
	Writable bool   `yaml:"writable,omitempty"`
}

type SSH struct {
	LocalPort int `yaml:"localPort,omitempty"` // REQUIRED (FIXME: auto assign)
}

type Firmware struct {
	// LegacyBIOS disables UEFI if set.
	// LegacyBIOS is ignored for aarch64.
	LegacyBIOS bool `yaml:"legacyBIOS,omitempty"`
}
