package limayaml

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/lima-vm/lima/pkg/guestagent/api"
	"github.com/lima-vm/lima/pkg/osutil"
	"github.com/lima-vm/lima/pkg/ptr"
	"github.com/lima-vm/lima/pkg/store/dirnames"
	"github.com/lima-vm/lima/pkg/store/filenames"
	"gotest.tools/v3/assert"
)

func TestFillDefault(t *testing.T) {
	var d, y, o LimaYAML

	opts := []cmp.Option{
		// Consider nil slices and empty slices to be identical
		cmpopts.EquateEmpty(),
	}

	var arch Arch
	switch runtime.GOARCH {
	case "amd64":
		arch = X8664
	case "arm64":
		arch = AARCH64
	case "arm":
		if runtime.GOOS != "linux" {
			t.Skipf("unsupported GOOS: %s", runtime.GOOS)
		}
		if arm := goarm(); arm < 7 {
			t.Skipf("unsupported GOARM: %d", arm)
		}
		arch = ARMV7L
	case "riscv64":
		arch = RISCV64
	default:
		t.Skipf("unknown GOARCH: %s", runtime.GOARCH)
	}

	hostHome, err := os.UserHomeDir()
	assert.NilError(t, err)
	limaHome, err := dirnames.LimaDir()
	assert.NilError(t, err)
	user, err := osutil.LimaUser(false)
	assert.NilError(t, err)

	guestHome := fmt.Sprintf("/home/%s.linux", user.Username)
	instName := "instance"
	instDir := filepath.Join(limaHome, instName)
	filePath := filepath.Join(instDir, filenames.LimaYAML)

	// Builtin default values
	builtin := LimaYAML{
		VMType: ptr.Of("qemu"),
		OS:     ptr.Of(LINUX),
		Arch:   ptr.Of(arch),
		CPUType: map[Arch]string{
			AARCH64: "cortex-a72",
			ARMV7L:  "cortex-a7",
			X8664:   "qemu64",
			RISCV64: "rv64",
		},
		CPUs:               ptr.Of(defaultCPUs()),
		Memory:             ptr.Of(defaultMemoryAsString()),
		Disk:               ptr.Of(defaultDiskSizeAsString()),
		GuestInstallPrefix: ptr.Of(defaultGuestInstallPrefix()),
		Containerd: Containerd{
			System:   ptr.Of(false),
			User:     ptr.Of(true),
			Archives: defaultContainerdArchives(),
		},
		SSH: SSH{
			LocalPort:         ptr.Of(0),
			LoadDotSSHPubKeys: ptr.Of(true),
			ForwardAgent:      ptr.Of(false),
			ForwardX11:        ptr.Of(false),
			ForwardX11Trusted: ptr.Of(false),
		},
		Firmware: Firmware{
			LegacyBIOS: ptr.Of(false),
		},
		Audio: Audio{
			Device: ptr.Of(""),
		},
		Video: Video{
			Display: ptr.Of("none"),
			VNC: VNCOptions{
				Display: ptr.Of("127.0.0.1:0,to=9"),
			},
		},
		HostResolver: HostResolver{
			Enabled: ptr.Of(true),
			IPv6:    ptr.Of(false),
		},
		PropagateProxyEnv: ptr.Of(true),
		CACertificates: CACertificates{
			RemoveDefaults: ptr.Of(false),
		},
		Plain: ptr.Of(false),
	}
	if IsAccelOS() {
		if HasHostCPU() {
			builtin.CPUType[arch] = "host"
		} else if HasMaxCPU() {
			builtin.CPUType[arch] = "max"
		}
		if arch == X8664 && runtime.GOOS == "darwin" {
			switch builtin.CPUType[arch] {
			case "host", "max":
				builtin.CPUType[arch] += ",-pdpe1gb"
			}
		}
	}

	defaultPortForward := PortForward{
		GuestIP:        api.IPv4loopback1,
		GuestPortRange: [2]int{1, 65535},
		HostIP:         api.IPv4loopback1,
		HostPortRange:  [2]int{1, 65535},
		Proto:          TCP,
		Reverse:        false,
	}

	// ------------------------------------------------------------------------------------
	// Builtin defaults are set when y is (mostly) empty

	// All these slices and maps are empty in "builtin". Add minimal entries here to see that
	// their values are retained and defaults for their fields are applied correctly.
	y = LimaYAML{
		HostResolver: HostResolver{
			Hosts: map[string]string{
				"MY.Host": "host.lima.internal",
			},
		},
		Mounts: []Mount{
			{Location: "/tmp"},
		},
		MountType: ptr.Of(NINEP),
		Provision: []Provision{
			{Script: "#!/bin/true"},
		},
		Probes: []Probe{
			{Script: "#!/bin/false"},
		},
		Networks: []Network{
			{Lima: "shared"},
		},
		DNS: []net.IP{
			net.ParseIP("1.0.1.0"),
		},
		PortForwards: []PortForward{
			{},
			{GuestPort: 80},
			{GuestPort: 8080, HostPort: 8888},
			{
				GuestSocket: "{{.Home}} | {{.UID}} | {{.User}}",
				HostSocket:  "{{.Home}} | {{.Dir}} | {{.Name}} | {{.UID}} | {{.User}}",
			},
		},
		CopyToHost: []CopyToHost{
			{
				GuestFile: "{{.Home}} | {{.UID}} | {{.User}}",
				HostFile:  "{{.Home}} | {{.Dir}} | {{.Name}} | {{.UID}} | {{.User}}",
			},
		},
		Env: map[string]string{
			"ONE": "Eins",
		},
		CACertificates: CACertificates{
			Files: []string{"ca.crt"},
			Certs: []string{
				"-----BEGIN CERTIFICATE-----\nYOUR-ORGS-TRUSTED-CA-CERT\n-----END CERTIFICATE-----\n",
			},
		},
	}

	expect := builtin
	expect.HostResolver.Hosts = map[string]string{
		"MY.Host": "host.lima.internal",
	}

	expect.Mounts = y.Mounts
	expect.Mounts[0].MountPoint = expect.Mounts[0].Location
	expect.Mounts[0].Writable = ptr.Of(false)
	expect.Mounts[0].SSHFS.Cache = ptr.Of(true)
	expect.Mounts[0].SSHFS.FollowSymlinks = ptr.Of(false)
	expect.Mounts[0].SSHFS.SFTPDriver = ptr.Of("")
	expect.Mounts[0].NineP.SecurityModel = ptr.Of(Default9pSecurityModel)
	expect.Mounts[0].NineP.ProtocolVersion = ptr.Of(Default9pProtocolVersion)
	expect.Mounts[0].NineP.Msize = ptr.Of(Default9pMsize)
	expect.Mounts[0].NineP.Cache = ptr.Of(Default9pCacheForRO)
	expect.Mounts[0].Virtiofs.QueueSize = ptr.Of(DefaultVirtiofsQueueSize)
	// Only missing Mounts field is Writable, and the default value is also the null value: false

	expect.MountType = ptr.Of(NINEP)

	expect.Provision = y.Provision
	expect.Provision[0].Mode = ProvisionModeSystem

	expect.Probes = y.Probes
	expect.Probes[0].Mode = ProbeModeReadiness
	expect.Probes[0].Description = "user probe 1/1"

	expect.Networks = y.Networks
	expect.Networks[0].MACAddress = MACAddress(fmt.Sprintf("%s#%d", filePath, 0))
	expect.Networks[0].Interface = "lima0"

	expect.DNS = y.DNS
	expect.PortForwards = []PortForward{
		defaultPortForward,
		defaultPortForward,
		defaultPortForward,
		defaultPortForward,
	}
	expect.CopyToHost = []CopyToHost{
		{},
	}

	// Setting GuestPort and HostPort for DeepEqual(), but they are not supposed to be used
	// after FillDefault() has been called and the ...PortRange fields have been set.
	expect.PortForwards[1].GuestPort = 80
	expect.PortForwards[1].GuestPortRange = [2]int{80, 80}
	expect.PortForwards[1].HostPortRange = expect.PortForwards[1].GuestPortRange

	expect.PortForwards[2].GuestPort = 8080
	expect.PortForwards[2].GuestPortRange = [2]int{8080, 8080}
	expect.PortForwards[2].HostPort = 8888
	expect.PortForwards[2].HostPortRange = [2]int{8888, 8888}

	expect.PortForwards[3].GuestSocket = fmt.Sprintf("%s | %s | %s", guestHome, user.Uid, user.Username)
	expect.PortForwards[3].HostSocket = fmt.Sprintf("%s | %s | %s | %s | %s", hostHome, instDir, instName, user.Uid, user.Username)

	expect.CopyToHost[0].GuestFile = fmt.Sprintf("%s | %s | %s", guestHome, user.Uid, user.Username)
	expect.CopyToHost[0].HostFile = fmt.Sprintf("%s | %s | %s | %s | %s", hostHome, instDir, instName, user.Uid, user.Username)

	expect.Env = y.Env

	expect.CACertificates = CACertificates{
		RemoveDefaults: ptr.Of(false),
		Files:          []string{"ca.crt"},
		Certs: []string{
			"-----BEGIN CERTIFICATE-----\nYOUR-ORGS-TRUSTED-CA-CERT\n-----END CERTIFICATE-----\n",
		},
	}

	expect.Rosetta = Rosetta{
		Enabled: ptr.Of(false),
		BinFmt:  ptr.Of(false),
	}

	FillDefault(&y, &LimaYAML{}, &LimaYAML{}, filePath)
	assert.DeepEqual(t, &y, &expect, opts...)

	filledDefaults := y

	// ------------------------------------------------------------------------------------
	// User-provided defaults should override any builtin defaults

	// Choose values that are different from the "builtin" defaults
	d = LimaYAML{
		VMType: ptr.Of("vz"),
		OS:     ptr.Of("unknown"),
		Arch:   ptr.Of("unknown"),
		CPUType: map[Arch]string{
			AARCH64: "arm64",
			ARMV7L:  "armhf",
			X8664:   "amd64",
			RISCV64: "riscv64",
		},
		CPUs:   ptr.Of(7),
		Memory: ptr.Of("5GiB"),
		Disk:   ptr.Of("105GiB"),
		AdditionalDisks: []Disk{
			{Name: "data"},
		},
		GuestInstallPrefix: ptr.Of("/opt"),
		Containerd: Containerd{
			System: ptr.Of(true),
			User:   ptr.Of(false),
			Archives: []File{
				{Location: "/tmp/nerdctl.tgz"},
			},
		},
		SSH: SSH{
			LocalPort:         ptr.Of(888),
			LoadDotSSHPubKeys: ptr.Of(false),
			ForwardAgent:      ptr.Of(true),
			ForwardX11:        ptr.Of(false),
			ForwardX11Trusted: ptr.Of(false),
		},
		Firmware: Firmware{
			LegacyBIOS: ptr.Of(true),
		},
		Audio: Audio{
			Device: ptr.Of("coreaudio"),
		},
		Video: Video{
			Display: ptr.Of("cocoa"),
			VNC: VNCOptions{
				Display: ptr.Of("none"),
			},
		},
		HostResolver: HostResolver{
			Enabled: ptr.Of(false),
			IPv6:    ptr.Of(true),
			Hosts: map[string]string{
				"default": "localhost",
			},
		},
		PropagateProxyEnv: ptr.Of(false),

		Mounts: []Mount{
			{
				Location: "/var/log",
				Writable: ptr.Of(false),
			},
		},
		Provision: []Provision{
			{
				Script: "#!/bin/true",
				Mode:   ProvisionModeUser,
			},
		},
		Probes: []Probe{
			{
				Script:      "#!/bin/false",
				Mode:        ProbeModeReadiness,
				Description: "User Probe",
			},
		},
		Networks: []Network{
			{
				VNLDeprecated:        "/tmp/vde.ctl",
				SwitchPortDeprecated: 65535,
				MACAddress:           "11:22:33:44:55:66",
				Interface:            "def0",
			},
		},
		DNS: []net.IP{
			net.ParseIP("1.1.1.1"),
		},
		PortForwards: []PortForward{{
			GuestIP:        api.IPv4loopback1,
			GuestPort:      80,
			GuestPortRange: [2]int{80, 80},
			HostIP:         api.IPv4loopback1,
			HostPort:       80,
			HostPortRange:  [2]int{80, 80},
			Proto:          TCP,
		}},
		CopyToHost: []CopyToHost{{}},
		Env: map[string]string{
			"ONE": "one",
			"TWO": "two",
		},
		CACertificates: CACertificates{
			RemoveDefaults: ptr.Of(true),
			Certs: []string{
				"-----BEGIN CERTIFICATE-----\nYOUR-ORGS-TRUSTED-CA-CERT\n-----END CERTIFICATE-----\n",
			},
		},
		Rosetta: Rosetta{
			Enabled: ptr.Of(true),
			BinFmt:  ptr.Of(true),
		},
	}

	expect = d
	// Also verify that archive arch is filled in
	expect.Containerd.Archives[0].Arch = *d.Arch
	expect.Mounts[0].MountPoint = expect.Mounts[0].Location
	expect.Mounts[0].SSHFS.Cache = ptr.Of(true)
	expect.Mounts[0].SSHFS.FollowSymlinks = ptr.Of(false)
	expect.Mounts[0].SSHFS.SFTPDriver = ptr.Of("")
	expect.Mounts[0].NineP.SecurityModel = ptr.Of(Default9pSecurityModel)
	expect.Mounts[0].NineP.ProtocolVersion = ptr.Of(Default9pProtocolVersion)
	expect.Mounts[0].NineP.Msize = ptr.Of(Default9pMsize)
	expect.Mounts[0].NineP.Cache = ptr.Of(Default9pCacheForRO)
	expect.Mounts[0].Virtiofs.QueueSize = ptr.Of(DefaultVirtiofsQueueSize)
	expect.HostResolver.Hosts = map[string]string{
		"default": d.HostResolver.Hosts["default"],
	}
	expect.MountType = ptr.Of(VIRTIOFS)
	expect.CACertificates.RemoveDefaults = ptr.Of(true)
	expect.CACertificates.Certs = []string{
		"-----BEGIN CERTIFICATE-----\nYOUR-ORGS-TRUSTED-CA-CERT\n-----END CERTIFICATE-----\n",
	}

	if runtime.GOOS == "darwin" && IsNativeArch(AARCH64) {
		expect.Rosetta = Rosetta{
			Enabled: ptr.Of(true),
			BinFmt:  ptr.Of(true),
		}
	} else {
		expect.Rosetta = Rosetta{
			Enabled: ptr.Of(false),
			BinFmt:  ptr.Of(true),
		}
	}
	expect.Plain = ptr.Of(false)

	y = LimaYAML{}
	FillDefault(&y, &d, &LimaYAML{}, filePath)
	assert.DeepEqual(t, &y, &expect, opts...)

	// ------------------------------------------------------------------------------------
	// User-provided defaults should not override user-provided config values

	y = filledDefaults
	y.DNS = []net.IP{net.ParseIP("8.8.8.8")}
	y.AdditionalDisks = []Disk{{Name: "overridden"}}

	expect = y

	expect.Provision = append(y.Provision, d.Provision...)
	expect.Probes = append(y.Probes, d.Probes...)
	expect.PortForwards = append(y.PortForwards, d.PortForwards...)
	expect.CopyToHost = append(y.CopyToHost, d.CopyToHost...)
	expect.Containerd.Archives = append(y.Containerd.Archives, d.Containerd.Archives...)
	expect.AdditionalDisks = append(y.AdditionalDisks, d.AdditionalDisks...)

	// Mounts and Networks start with lowest priority first, so higher priority entries can overwrite
	expect.Mounts = append(d.Mounts, y.Mounts...)
	expect.Networks = append(d.Networks, y.Networks...)

	expect.HostResolver.Hosts["default"] = d.HostResolver.Hosts["default"]

	// d.DNS will be ignored, and not appended to y.DNS

	// "TWO" does not exist in filledDefaults.Env, so is set from d.Env
	expect.Env["TWO"] = d.Env["TWO"]

	FillDefault(&y, &d, &LimaYAML{}, filePath)
	assert.DeepEqual(t, &y, &expect, opts...)

	// ------------------------------------------------------------------------------------
	// User-provided overrides should override user-provided config settings

	o = LimaYAML{
		VMType: ptr.Of("qemu"),
		OS:     ptr.Of(LINUX),
		Arch:   ptr.Of(arch),
		CPUType: map[Arch]string{
			AARCH64: "uber-arm",
			ARMV7L:  "armv8",
			X8664:   "pentium",
			RISCV64: "sifive-u54",
		},
		CPUs:   ptr.Of(12),
		Memory: ptr.Of("7GiB"),
		Disk:   ptr.Of("117GiB"),
		AdditionalDisks: []Disk{
			{Name: "test"},
		},
		GuestInstallPrefix: ptr.Of("/usr"),
		Containerd: Containerd{
			System: ptr.Of(true),
			User:   ptr.Of(false),
			Archives: []File{
				{
					Arch:     arch,
					Location: "/tmp/nerdctl.tgz",
					Digest:   "$DIGEST",
				},
			},
		},
		SSH: SSH{
			LocalPort:         ptr.Of(4433),
			LoadDotSSHPubKeys: ptr.Of(true),
			ForwardAgent:      ptr.Of(true),
			ForwardX11:        ptr.Of(false),
			ForwardX11Trusted: ptr.Of(false),
		},
		Firmware: Firmware{
			LegacyBIOS: ptr.Of(true),
		},
		Audio: Audio{
			Device: ptr.Of("coreaudio"),
		},
		Video: Video{
			Display: ptr.Of("cocoa"),
			VNC: VNCOptions{
				Display: ptr.Of("none"),
			},
		},
		HostResolver: HostResolver{
			Enabled: ptr.Of(false),
			IPv6:    ptr.Of(false),
			Hosts: map[string]string{
				"override.": "underflow",
			},
		},
		PropagateProxyEnv: ptr.Of(false),

		Mounts: []Mount{
			{
				Location: "/var/log",
				Writable: ptr.Of(true),
				SSHFS: SSHFS{
					Cache:          ptr.Of(false),
					FollowSymlinks: ptr.Of(true),
				},
				NineP: NineP{
					SecurityModel:   ptr.Of("mapped-file"),
					ProtocolVersion: ptr.Of("9p2000"),
					Msize:           ptr.Of("8KiB"),
					Cache:           ptr.Of("none"),
				},
				Virtiofs: Virtiofs{
					QueueSize: ptr.Of(2048),
				},
			},
		},
		Provision: []Provision{
			{
				Script: "#!/bin/true",
				Mode:   ProvisionModeSystem,
			},
		},
		Probes: []Probe{
			{
				Script:      "#!/bin/false",
				Mode:        ProbeModeReadiness,
				Description: "Another Probe",
			},
		},
		Networks: []Network{
			{
				Lima:       "shared",
				MACAddress: "10:20:30:40:50:60",
				Interface:  "def1",
			},
			{
				Lima:      "bridged",
				Interface: "def0",
			},
		},
		DNS: []net.IP{
			net.ParseIP("2.2.2.2"),
		},
		PortForwards: []PortForward{{
			GuestIP:        api.IPv4loopback1,
			GuestPort:      88,
			GuestPortRange: [2]int{88, 88},
			HostIP:         api.IPv4loopback1,
			HostPort:       8080,
			HostPortRange:  [2]int{8080, 8080},
			Proto:          TCP,
		}},
		CopyToHost: []CopyToHost{{}},
		Env: map[string]string{
			"TWO":   "deux",
			"THREE": "trois",
		},
		CACertificates: CACertificates{
			RemoveDefaults: ptr.Of(true),
		},
		Rosetta: Rosetta{
			Enabled: ptr.Of(false),
			BinFmt:  ptr.Of(false),
		},
	}

	y = filledDefaults

	expect = o

	expect.Provision = append(append(o.Provision, y.Provision...), d.Provision...)
	expect.Probes = append(append(o.Probes, y.Probes...), d.Probes...)
	expect.PortForwards = append(append(o.PortForwards, y.PortForwards...), d.PortForwards...)
	expect.CopyToHost = append(append(o.CopyToHost, y.CopyToHost...), d.CopyToHost...)
	expect.Containerd.Archives = append(append(o.Containerd.Archives, y.Containerd.Archives...), d.Containerd.Archives...)
	expect.AdditionalDisks = append(append(o.AdditionalDisks, y.AdditionalDisks...), d.AdditionalDisks...)

	expect.HostResolver.Hosts["default"] = d.HostResolver.Hosts["default"]
	expect.HostResolver.Hosts["MY.Host"] = d.HostResolver.Hosts["host.lima.internal"]

	// o.Mounts just makes d.Mounts[0] writable because the Location matches
	expect.Mounts = append(d.Mounts, y.Mounts...)
	expect.Mounts[0].Writable = ptr.Of(true)
	expect.Mounts[0].SSHFS.Cache = ptr.Of(false)
	expect.Mounts[0].SSHFS.FollowSymlinks = ptr.Of(true)
	expect.Mounts[0].NineP.SecurityModel = ptr.Of("mapped-file")
	expect.Mounts[0].NineP.ProtocolVersion = ptr.Of("9p2000")
	expect.Mounts[0].NineP.Msize = ptr.Of("8KiB")
	expect.Mounts[0].NineP.Cache = ptr.Of("none")
	expect.Mounts[0].Virtiofs.QueueSize = ptr.Of(2048)

	expect.MountType = ptr.Of(NINEP)

	// o.Networks[1] is overriding the d.Networks[0].Lima entry for the "def0" interface
	expect.Networks = append(append(d.Networks, y.Networks...), o.Networks[0])
	expect.Networks[0].Lima = o.Networks[1].Lima
	expect.Networks[0].VNLDeprecated = ""
	expect.Networks[0].SwitchPortDeprecated = 0

	// Only highest prio DNS are retained
	expect.DNS = o.DNS

	// ONE remains from filledDefaults.Env; the rest are set from o
	expect.Env["ONE"] = y.Env["ONE"]

	expect.CACertificates.RemoveDefaults = ptr.Of(true)
	expect.CACertificates.Files = []string{"ca.crt"}
	expect.CACertificates.Certs = []string{
		"-----BEGIN CERTIFICATE-----\nYOUR-ORGS-TRUSTED-CA-CERT\n-----END CERTIFICATE-----\n",
	}

	expect.Rosetta = Rosetta{
		Enabled: ptr.Of(false),
		BinFmt:  ptr.Of(false),
	}
	expect.Plain = ptr.Of(false)

	FillDefault(&y, &d, &o, filePath)
	assert.DeepEqual(t, &y, &expect, opts...)
}
