// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package limayaml

import (
	"fmt"
	"net"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/sirupsen/logrus"
	"gotest.tools/v3/assert"

	"github.com/lima-vm/lima/v2/pkg/ioutilx"
	"github.com/lima-vm/lima/v2/pkg/limatype"
	"github.com/lima-vm/lima/v2/pkg/limatype/dirnames"
	"github.com/lima-vm/lima/v2/pkg/limatype/filenames"
	"github.com/lima-vm/lima/v2/pkg/osutil"
	"github.com/lima-vm/lima/v2/pkg/ptr"
	"github.com/lima-vm/lima/v2/pkg/version"
)

func TestExistingLimaVersion(t *testing.T) {
	t.Run("instance does not exist yet", func(t *testing.T) {
		assert.Equal(t, ExistingLimaVersion(t.TempDir()), version.Version)
	})

	t.Run("version has been recorded", func(t *testing.T) {
		instDir := createInstanceDir(t, map[string]string{filenames.LimaVersion: "1.2.3\n"})
		assert.Equal(t, ExistingLimaVersion(instDir), "1.2.3")
	})

	// Instances created before Lima v0.20 have no lima-version file. Reporting the current
	// version for them would move the guest home directory to a path that doesn't exist.
	t.Run("instance predates the version file", func(t *testing.T) {
		instDir := createInstanceDir(t, nil)
		assert.Equal(t, ExistingLimaVersion(instDir), "")

		user := osutil.LimaUser(t.Context(), ExistingLimaVersion(instDir), false, nil)
		assert.Equal(t, user.HomeDir, "/home/{{.User}}.linux")
	})
}

// createInstanceDir returns a directory that IsExistingInstanceDir recognizes as an
// existing instance, populated with files.
func createInstanceDir(t *testing.T, files map[string]string) string {
	t.Helper()
	instDir := t.TempDir()
	// The host agent log is one of the files that identify an existing instance.
	assert.NilError(t, os.WriteFile(filepath.Join(instDir, filenames.HostAgentStdoutLog), nil, 0o644))
	for name, content := range files {
		assert.NilError(t, os.WriteFile(filepath.Join(instDir, name), []byte(content), 0o644))
	}
	return instDir
}

func TestFillDefault(t *testing.T) {
	logrus.SetLevel(logrus.DebugLevel)
	var d, y, o limatype.LimaYAML

	opts := []cmp.Option{
		// Consider nil slices and empty slices to be identical
		cmpopts.EquateEmpty(),
	}

	var arch limatype.Arch
	switch runtime.GOARCH {
	case "amd64":
		arch = limatype.X8664
	case "arm64":
		arch = limatype.AARCH64
	case "arm":
		if runtime.GOOS != "linux" {
			t.Skipf("unsupported GOOS: %s", runtime.GOOS)
		}
		if arm := limatype.Goarm(); arm < 7 {
			t.Skipf("unsupported GOARM: %d", arm)
		}
		arch = limatype.ARMV7L
	case "ppc64le":
		arch = limatype.PPC64LE
	case "riscv64":
		arch = limatype.RISCV64
	case "s390x":
		arch = limatype.S390X
	default:
		t.Skipf("unknown GOARCH: %s", runtime.GOARCH)
	}

	hostHome, err := os.UserHomeDir()
	assert.NilError(t, err)
	limaHome, err := dirnames.LimaDir()
	assert.NilError(t, err)
	user := osutil.LimaUser(t.Context(), "0.0.0", false, nil)
	user.HomeDir = fmt.Sprintf("/home/%s.guest", user.Username)
	uid, err := strconv.ParseUint(user.Uid, 10, 32)
	assert.NilError(t, err)

	instName := "instance"
	instDir := filepath.Join(limaHome, instName)
	filePath := filepath.Join(instDir, filenames.LimaYAML)

	// Builtin default values
	builtin := limatype.LimaYAML{
		OS:                 ptr.Of(limatype.LINUX),
		Arch:               ptr.Of(arch),
		CPUs:               ptr.Of(defaultCPUs()),
		Memory:             ptr.Of(defaultMemoryAsString()),
		Disk:               ptr.Of(defaultDiskSizeAsString()),
		GuestInstallPrefix: ptr.Of(defaultGuestInstallPrefix()),
		UpgradePackages:    ptr.Of(false),
		Containerd: limatype.Containerd{
			System:   ptr.Of(false),
			User:     ptr.Of(true),
			Archives: defaultContainerdArchives(),
		},
		SSH: limatype.SSH{
			LocalPort:         ptr.Of(0),
			LoadDotSSHPubKeys: ptr.Of(false),
			ForwardAgent:      ptr.Of(false),
			ForwardX11:        ptr.Of(false),
			ForwardX11Trusted: ptr.Of(false),
		},
		TimeZone: ptr.Of(hostTimeZone()),
		Firmware: limatype.Firmware{
			LegacyBIOS: ptr.Of(false),
		},
		Audio: limatype.Audio{
			Device:    ptr.Of(""),
			Interface: ptr.Of(""),
		},
		Video: limatype.Video{
			Display: ptr.Of("none"),
		},
		HostResolver: limatype.HostResolver{
			Enabled: ptr.Of(true),
			IPv6:    ptr.Of(false),
		},
		PropagateProxyEnv: ptr.Of(true),
		CACertificates: limatype.CACertificates{
			RemoveDefaults: ptr.Of(false),
		},
		NestedVirtualization: ptr.Of(false),
		Plain:                ptr.Of(false),
		User: limatype.User{
			Name:             ptr.Of(user.Username),
			Comment:          ptr.Of(user.Name),
			Home:             ptr.Of(user.HomeDir),
			Shell:            ptr.Of("/bin/bash"),
			UID:              ptr.Of(uint32(uid)),
			PasswordlessSudo: ptr.Of(true),
		},
		TPM: ptr.Of(false),
	}

	defaultPortForward := limatype.PortForward{
		GuestIP:           IPv4loopback1,
		GuestIPMustBeZero: ptr.Of(false),
		GuestPortRange:    [2]int{1, 65535},
		HostIP:            IPv4loopback1,
		HostPortRange:     [2]int{1, 65535},
		Proto:             limatype.ProtoAny,
		Reverse:           false,
	}

	// ------------------------------------------------------------------------------------
	// Builtin defaults are set when y is (mostly) empty

	// All these slices and maps are empty in "builtin". Add minimal entries here to see that
	// their values are retained and defaults for their fields are applied correctly.
	y = limatype.LimaYAML{
		HostResolver: limatype.HostResolver{
			Hosts: map[string]string{
				"MY.Host": "host.lima.internal",
			},
		},
		Mounts: []limatype.Mount{
			//nolint:usetesting // We need the OS temp directory name here; it is not used to create temp files for testing
			{Location: filepath.Clean(os.TempDir())},
			{Location: filepath.Clean("{{.Dir}}/{{.Param.ONE}}"), MountPoint: ptr.Of("/mnt/{{.Param.ONE}}")},
		},
		MountType: ptr.Of(limatype.NINEP),
		Provision: []limatype.Provision{
			{Script: ptr.Of("#!/bin/true # {{.Param.ONE}}")},
		},
		Probes: []limatype.Probe{
			{Script: ptr.Of("#!/bin/false # {{.Param.ONE}}")},
		},
		Networks: []limatype.Network{
			{Lima: "shared"},
		},
		DNS: []net.IP{
			net.ParseIP("1.0.1.0"),
		},
		PortForwards: []limatype.PortForward{
			{},
			{GuestPort: 80},
			{GuestPort: 8080, HostPort: 8888},
			{
				GuestSocket: "{{.Home}} | {{.UID}} | {{.User}} | {{.Param.ONE}}",
				HostSocket:  "{{.Home}} | {{.Dir}} | {{.Name}} | {{.UID}} | {{.User}} | {{.Param.ONE}}",
			},
		},
		CopyToHost: []limatype.CopyToHost{
			{
				GuestFile: "{{.Home}} | {{.UID}} | {{.User}} | {{.Param.ONE}}",
				HostFile:  "{{.Home}} | {{.Dir}} | {{.Name}} | {{.UID}} | {{.User}} | {{.Param.ONE}}",
			},
		},
		Env: map[string]string{
			"ONE": "Eins",
		},
		Param: map[string]string{
			"ONE": "Eins",
		},
		CACertificates: limatype.CACertificates{
			Files: []string{"ca.crt"},
			Certs: []string{
				"-----BEGIN CERTIFICATE-----\nYOUR-ORGS-TRUSTED-CA-CERT\n-----END CERTIFICATE-----\n",
			},
		},
		TimeZone: ptr.Of("Antarctica/Troll"),
	}

	expect := builtin
	// VMType should remain nil when not explicitly set (will be resolved by ValidateVMType later)
	expect.VMType = nil
	expect.HostResolver.Hosts = map[string]string{
		"MY.Host": "host.lima.internal",
	}

	expect.Mounts = slices.Clone(y.Mounts)
	expect.Mounts[0].MountPoint = ptr.Of(expect.Mounts[0].Location)
	if runtime.GOOS == "windows" {
		mountLocation, err := ioutilx.WindowsSubsystemPath(t.Context(), expect.Mounts[0].Location)
		if err == nil {
			expect.Mounts[0].MountPoint = ptr.Of(mountLocation)
		}
	}
	expect.Mounts[0].Writable = ptr.Of(false)
	expect.Mounts[0].SSHFS.Cache = ptr.Of(true)
	expect.Mounts[0].SSHFS.FollowSymlinks = ptr.Of(false)
	expect.Mounts[0].SSHFS.SFTPDriver = ptr.Of("")
	expect.Mounts[0].NineP.SecurityModel = ptr.Of(Default9pSecurityModel)
	expect.Mounts[0].NineP.ProtocolVersion = ptr.Of(Default9pProtocolVersion)
	expect.Mounts[0].NineP.Msize = ptr.Of(Default9pMsize)
	expect.Mounts[0].NineP.Cache = ptr.Of(Default9pCacheForRO)
	expect.Mounts[0].Virtiofs.QueueSize = nil
	// Only missing Mounts field is Writable, and the default value is also the null value: false
	expect.Mounts[1].Location = filepath.Join(instDir, y.Param["ONE"])
	expect.Mounts[1].MountPoint = ptr.Of(path.Join("/mnt", y.Param["ONE"]))
	expect.Mounts[1].Writable = ptr.Of(false)
	expect.Mounts[1].SSHFS.Cache = ptr.Of(true)
	expect.Mounts[1].SSHFS.FollowSymlinks = ptr.Of(false)
	expect.Mounts[1].SSHFS.SFTPDriver = ptr.Of("")
	expect.Mounts[1].NineP.SecurityModel = ptr.Of(Default9pSecurityModel)
	expect.Mounts[1].NineP.ProtocolVersion = ptr.Of(Default9pProtocolVersion)
	expect.Mounts[1].NineP.Msize = ptr.Of(Default9pMsize)
	expect.Mounts[1].NineP.Cache = ptr.Of(Default9pCacheForRO)
	expect.Mounts[1].Virtiofs.QueueSize = nil

	expect.MountType = ptr.Of(limatype.NINEP)

	expect.MountInotify = ptr.Of(false)

	expect.Provision = slices.Clone(y.Provision)
	expect.Provision[0].Mode = limatype.ProvisionModeSystem
	expect.Provision[0].Script = ptr.Of("#!/bin/true # Eins")

	expect.Probes = slices.Clone(y.Probes)
	expect.Probes[0].Mode = limatype.ProbeModeReadiness
	expect.Probes[0].Description = "user probe 1/1"
	expect.Probes[0].Script = ptr.Of("#!/bin/false # Eins")

	expect.Networks = slices.Clone(y.Networks)
	expect.Networks[0].MACAddress = MACAddress(fmt.Sprintf("%s#%d", filePath, 0))
	expect.Networks[0].Interface = "lima0"
	expect.Networks[0].Metric = ptr.Of(uint32(100))

	expect.DNS = slices.Clone(y.DNS)
	expect.PortForwards = []limatype.PortForward{
		defaultPortForward,
		defaultPortForward,
		defaultPortForward,
		defaultPortForward,
	}
	expect.CopyToHost = []limatype.CopyToHost{
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

	expect.PortForwards[3].HostPortRange = [2]int{0, 0}
	expect.PortForwards[3].GuestSocket = fmt.Sprintf("%s | %s | %s | %s", user.HomeDir, user.Uid, user.Username, y.Param["ONE"])
	expect.PortForwards[3].HostSocket = fmt.Sprintf("%s | %s | %s | %s | %s | %s", hostHome, instDir, instName, currentUser.Uid, currentUser.Username, y.Param["ONE"])

	expect.CopyToHost[0].GuestFile = fmt.Sprintf("%s | %s | %s | %s", user.HomeDir, user.Uid, user.Username, y.Param["ONE"])
	expect.CopyToHost[0].HostFile = fmt.Sprintf("%s | %s | %s | %s | %s | %s", hostHome, instDir, instName, currentUser.Uid, currentUser.Username, y.Param["ONE"])

	expect.Env = y.Env

	expect.Param = y.Param

	expect.CACertificates = limatype.CACertificates{
		RemoveDefaults: ptr.Of(false),
		Files:          []string{"ca.crt"},
		Certs: []string{
			"-----BEGIN CERTIFICATE-----\nYOUR-ORGS-TRUSTED-CA-CERT\n-----END CERTIFICATE-----\n",
		},
	}

	expect.TimeZone = y.TimeZone
	// Set firmware expectations to match what FillDefault actually does
	// FillDefault uses the builtin default values, which include LegacyBIOS: ptr.Of(false)
	expect.Firmware = limatype.Firmware{
		LegacyBIOS: ptr.Of(false), // This matches what FillDefault actually sets
		Images:     nil,
	}

	expect.NestedVirtualization = ptr.Of(false)
	expect.TPM = ptr.Of(false)

	FillDefault(t.Context(), &y, &limatype.LimaYAML{}, &limatype.LimaYAML{}, filePath, false)
	assert.DeepEqual(t, &y, &expect, opts...)

	filledDefaults := y

	// ------------------------------------------------------------------------------------
	// User-provided defaults should override any builtin defaults

	// Choose values that are different from the "builtin" defaults

	// Calling filepath.Abs() to add a drive letter on Windows
	varLog, _ := filepath.Abs("/var/log")
	d = limatype.LimaYAML{
		// Remove driver-specific VMType from defaults test
		OS:     ptr.Of("unknown"),
		Arch:   ptr.Of("unknown"),
		CPUs:   ptr.Of(7),
		Memory: ptr.Of("5GiB"),
		Disk:   ptr.Of("105GiB"),
		AdditionalDisks: []limatype.Disk{
			{Name: "data"},
		},
		GuestInstallPrefix: ptr.Of("/opt"),
		UpgradePackages:    ptr.Of(true),
		Containerd: limatype.Containerd{
			System: ptr.Of(true),
			User:   ptr.Of(false),
			Archives: []limatype.File{
				{Location: "/tmp/nerdctl.tgz"},
			},
		},
		SSH: limatype.SSH{
			LocalPort:         ptr.Of(888),
			LoadDotSSHPubKeys: ptr.Of(false),
			ForwardAgent:      ptr.Of(true),
			ForwardX11:        ptr.Of(false),
			ForwardX11Trusted: ptr.Of(false),
		},
		TimeZone: ptr.Of("Zulu"),
		Firmware: limatype.Firmware{
			LegacyBIOS: ptr.Of(true),
			// Remove driver-specific firmware images from defaults
		},
		Audio: limatype.Audio{
			Device:    ptr.Of("coreaudio"),
			Interface: ptr.Of("virtio"),
		},
		Video: limatype.Video{
			Display: ptr.Of("cocoa"),
			// Remove driver-specific VNC configuration
		},
		HostResolver: limatype.HostResolver{
			Enabled: ptr.Of(false),
			IPv6:    ptr.Of(true),
			Hosts: map[string]string{
				"default": "localhost",
			},
		},
		PropagateProxyEnv: ptr.Of(false),

		Mounts: []limatype.Mount{
			{
				Location: varLog,
				Writable: ptr.Of(false),
			},
		},
		Provision: []limatype.Provision{
			{
				Script: ptr.Of("#!/bin/true"),
				Mode:   limatype.ProvisionModeUser,
			},
		},
		Probes: []limatype.Probe{
			{
				Script:      ptr.Of("#!/bin/false"),
				Mode:        limatype.ProbeModeReadiness,
				Description: "User Probe",
			},
		},
		Networks: []limatype.Network{
			{
				MACAddress: "11:22:33:44:55:66",
				Interface:  "def0",
				Metric:     ptr.Of(uint32(50)),
			},
		},
		DNS: []net.IP{
			net.ParseIP("1.1.1.1"),
		},
		PortForwards: []limatype.PortForward{{
			GuestIP:           IPv4loopback1,
			GuestIPMustBeZero: ptr.Of(false),
			GuestPort:         80,
			GuestPortRange:    [2]int{80, 80},
			HostIP:            IPv4loopback1,
			HostPort:          80,
			HostPortRange:     [2]int{80, 80},
			Proto:             limatype.ProtoTCP,
		}},
		CopyToHost: []limatype.CopyToHost{{}},
		Env: map[string]string{
			"ONE": "one",
			"TWO": "two",
		},
		Param: map[string]string{
			"ONE": "one",
			"TWO": "two",
		},
		CACertificates: limatype.CACertificates{
			RemoveDefaults: ptr.Of(true),
			Certs: []string{
				"-----BEGIN CERTIFICATE-----\nYOUR-ORGS-TRUSTED-CA-CERT\n-----END CERTIFICATE-----\n",
			},
		},
		NestedVirtualization: ptr.Of(true),
		User: limatype.User{
			Name:             ptr.Of("xxx"),
			Comment:          ptr.Of("Foo Bar"),
			Home:             ptr.Of("/tmp"),
			Shell:            ptr.Of("/bin/tcsh"),
			UID:              ptr.Of(uint32(8080)),
			PasswordlessSudo: ptr.Of(false),
		},
		VMOpts: limatype.VMOpts{
			"qemu": map[string]any{
				"minimumVersion": "9.1.0",
			},
		},
		TPM: ptr.Of(true),
	}

	expect = d
	// VMType should remain nil when not explicitly set
	expect.VMType = nil
	// Also verify that archive arch is filled in
	expect.Containerd.Archives = slices.Clone(d.Containerd.Archives)
	expect.Containerd.Archives[0].Arch = *d.Arch
	expect.Mounts = slices.Clone(d.Mounts)
	expect.Mounts[0].MountPoint = ptr.Of(expect.Mounts[0].Location)
	if runtime.GOOS == "windows" {
		mountLocation, err := ioutilx.WindowsSubsystemPath(t.Context(), expect.Mounts[0].Location)
		if err == nil {
			expect.Mounts[0].MountPoint = ptr.Of(mountLocation)
		}
	}
	expect.Mounts[0].SSHFS.Cache = ptr.Of(true)
	expect.Mounts[0].SSHFS.FollowSymlinks = ptr.Of(false)
	expect.Mounts[0].SSHFS.SFTPDriver = ptr.Of("")
	expect.Mounts[0].NineP.SecurityModel = ptr.Of(Default9pSecurityModel)
	expect.Mounts[0].NineP.ProtocolVersion = ptr.Of(Default9pProtocolVersion)
	expect.Mounts[0].NineP.Msize = ptr.Of(Default9pMsize)
	expect.Mounts[0].NineP.Cache = ptr.Of(Default9pCacheForRO)
	expect.Mounts[0].Virtiofs.QueueSize = nil
	expect.HostResolver.Hosts = map[string]string{
		"default": d.HostResolver.Hosts["default"],
	}
	// Remove driver-specific mount type from defaults test
	expect.MountType = nil
	expect.MountInotify = ptr.Of(false)
	expect.CACertificates.RemoveDefaults = ptr.Of(true)
	expect.CACertificates.Certs = []string{
		"-----BEGIN CERTIFICATE-----\nYOUR-ORGS-TRUSTED-CA-CERT\n-----END CERTIFICATE-----\n",
	}

	expect.Plain = ptr.Of(false)
	expect.TPM = ptr.Of(true)

	y = limatype.LimaYAML{}
	FillDefault(t.Context(), &y, &d, &limatype.LimaYAML{}, filePath, false)
	assert.DeepEqual(t, &y, &expect, opts...)

	dExpected := expect

	// ------------------------------------------------------------------------------------
	// User-provided defaults should not override user-provided config values

	y = filledDefaults
	y.DNS = []net.IP{net.ParseIP("8.8.8.8")}
	y.AdditionalDisks = []limatype.Disk{{Name: "overridden"}}
	y.User.Home = ptr.Of("/root")
	y.VMOpts = limatype.VMOpts{
		"vz": map[string]any{
			"diskImageFormat": "raw",
		},
	}

	expect = y

	expect.Provision = slices.Concat(y.Provision, dExpected.Provision)
	expect.Probes = slices.Concat(y.Probes, dExpected.Probes)
	expect.PortForwards = slices.Concat(y.PortForwards, dExpected.PortForwards)
	expect.CopyToHost = slices.Concat(y.CopyToHost, dExpected.CopyToHost)
	expect.Containerd.Archives = slices.Concat(y.Containerd.Archives, dExpected.Containerd.Archives)
	expect.Containerd.Archives[2].Arch = *expect.Arch
	expect.AdditionalDisks = slices.Concat(y.AdditionalDisks, dExpected.AdditionalDisks)
	expect.Firmware.Images = slices.Concat(y.Firmware.Images, dExpected.Firmware.Images)

	// Mounts and Networks start with lowest priority first, so higher priority entries can overwrite
	expect.Mounts = slices.Concat(dExpected.Mounts, y.Mounts)
	expect.Networks = slices.Concat(dExpected.Networks, y.Networks)

	expect.HostResolver.Hosts["default"] = dExpected.HostResolver.Hosts["default"]

	// dExpected.DNS will be ignored, and not appended to y.DNS

	// "TWO" does not exist in filledDefaults.Env, so is set from dExpected.Env
	expect.Env["TWO"] = dExpected.Env["TWO"]

	expect.Param["TWO"] = dExpected.Param["TWO"]

	expect.VMOpts = limatype.VMOpts{
		"qemu": dExpected.VMOpts["qemu"],
		"vz":   y.VMOpts["vz"],
	}

	t.Logf("d.vmType=%v, y.vmType=%v, expect.vmType=%v", d.VMType, y.VMType, expect.VMType)

	FillDefault(t.Context(), &y, &d, &limatype.LimaYAML{}, filePath, false)
	assert.DeepEqual(t, &y, &expect, opts...)

	// ------------------------------------------------------------------------------------
	// User-provided overrides should override user-provided config settings

	o = limatype.LimaYAML{
		// Remove driver-specific VMType from override test
		OS:     ptr.Of(limatype.LINUX),
		Arch:   ptr.Of(arch),
		CPUs:   ptr.Of(12),
		Memory: ptr.Of("7GiB"),
		Disk:   ptr.Of("117GiB"),
		AdditionalDisks: []limatype.Disk{
			{Name: "test"},
		},
		GuestInstallPrefix: ptr.Of("/usr"),
		UpgradePackages:    ptr.Of(true),
		Containerd: limatype.Containerd{
			System: ptr.Of(true),
			User:   ptr.Of(false),
			Archives: []limatype.File{
				{
					Arch:     arch,
					Location: "/tmp/nerdctl.tgz",
					Digest:   "$DIGEST",
				},
			},
		},
		SSH: limatype.SSH{
			LocalPort:         ptr.Of(4433),
			LoadDotSSHPubKeys: ptr.Of(true),
			ForwardAgent:      ptr.Of(true),
			ForwardX11:        ptr.Of(false),
			ForwardX11Trusted: ptr.Of(false),
		},
		TimeZone: ptr.Of("Universal"),
		Firmware: limatype.Firmware{
			LegacyBIOS: ptr.Of(true),
		},
		Audio: limatype.Audio{
			Device:    ptr.Of("coreaudio"),
			Interface: ptr.Of("hda"),
		},
		Video: limatype.Video{
			Display: ptr.Of("cocoa"),
			// Remove driver-specific VNC configuration
		},
		HostResolver: limatype.HostResolver{
			Enabled: ptr.Of(false),
			IPv6:    ptr.Of(false),
			Hosts: map[string]string{
				"override.": "underflow",
			},
		},
		PropagateProxyEnv: ptr.Of(false),

		Mounts: []limatype.Mount{
			{
				Location: varLog,
				Writable: ptr.Of(true),
				SSHFS: limatype.SSHFS{
					Cache:          ptr.Of(false),
					FollowSymlinks: ptr.Of(true),
				},
				NineP: limatype.NineP{
					SecurityModel:   ptr.Of("mapped-file"),
					ProtocolVersion: ptr.Of("9p2000"),
					Msize:           ptr.Of("8KiB"),
					Cache:           ptr.Of("none"),
				},
				Virtiofs: limatype.Virtiofs{
					QueueSize: ptr.Of(2048),
				},
			},
		},
		MountInotify: ptr.Of(true),
		Provision: []limatype.Provision{
			{
				Script: ptr.Of("#!/bin/true"),
				Mode:   limatype.ProvisionModeSystem,
			},
		},
		Probes: []limatype.Probe{
			{
				Script:      ptr.Of("#!/bin/false"),
				Mode:        limatype.ProbeModeReadiness,
				Description: "Another Probe",
			},
		},
		Networks: []limatype.Network{
			{
				Lima:       "shared",
				MACAddress: "10:20:30:40:50:60",
				Interface:  "def1",
				Metric:     ptr.Of(uint32(25)),
			},
			{
				Lima:      "bridged",
				Interface: "def0",
			},
		},
		DNS: []net.IP{
			net.ParseIP("2.2.2.2"),
		},
		PortForwards: []limatype.PortForward{{
			GuestIP:           IPv4loopback1,
			GuestIPMustBeZero: ptr.Of(false),
			GuestPort:         88,
			GuestPortRange:    [2]int{88, 88},
			HostIP:            IPv4loopback1,
			HostPort:          8080,
			HostPortRange:     [2]int{8080, 8080},
			Proto:             limatype.ProtoTCP,
		}},
		CopyToHost: []limatype.CopyToHost{{}},
		Env: map[string]string{
			"TWO":   "deux",
			"THREE": "trois",
		},
		Param: map[string]string{
			"TWO":   "deux",
			"THREE": "trois",
		},
		CACertificates: limatype.CACertificates{
			RemoveDefaults: ptr.Of(true),
		},
		NestedVirtualization: ptr.Of(false),
		User: limatype.User{
			Name:             ptr.Of("foo"),
			Comment:          ptr.Of("foo bar baz"),
			Home:             ptr.Of("/override"),
			Shell:            ptr.Of("/bin/sh"),
			UID:              ptr.Of(uint32(1122)),
			PasswordlessSudo: ptr.Of(true),
		},
		VMOpts: limatype.VMOpts{
			"vz": map[string]any{
				"rosetta": map[string]any{
					"enabled": true,
					"binfmt":  true,
				},
			},
		},
	}

	y = filledDefaults

	expect = o

	expect.Provision = slices.Concat(o.Provision, y.Provision, dExpected.Provision)
	expect.Probes = slices.Concat(o.Probes, y.Probes, dExpected.Probes)
	expect.PortForwards = slices.Concat(o.PortForwards, y.PortForwards, dExpected.PortForwards)
	expect.CopyToHost = slices.Concat(o.CopyToHost, y.CopyToHost, dExpected.CopyToHost)
	expect.Containerd.Archives = slices.Concat(o.Containerd.Archives, y.Containerd.Archives, dExpected.Containerd.Archives)
	expect.Containerd.Archives[3].Arch = *expect.Arch
	expect.AdditionalDisks = slices.Concat(o.AdditionalDisks, y.AdditionalDisks, dExpected.AdditionalDisks)
	expect.Firmware.Images = slices.Concat(o.Firmware.Images, y.Firmware.Images, dExpected.Firmware.Images)

	expect.HostResolver.Hosts["default"] = dExpected.HostResolver.Hosts["default"]
	expect.HostResolver.Hosts["MY.Host"] = dExpected.HostResolver.Hosts["host.lima.internal"]

	// o.Mounts just makes dExpected.Mounts[0] writable because the Location matches
	expect.Mounts = slices.Concat(dExpected.Mounts, y.Mounts)
	expect.Mounts[0].Writable = ptr.Of(true)
	expect.Mounts[0].SSHFS.Cache = ptr.Of(false)
	expect.Mounts[0].SSHFS.FollowSymlinks = ptr.Of(true)
	expect.Mounts[0].NineP.SecurityModel = ptr.Of("mapped-file")
	expect.Mounts[0].NineP.ProtocolVersion = ptr.Of("9p2000")
	expect.Mounts[0].NineP.Msize = ptr.Of("8KiB")
	expect.Mounts[0].NineP.Cache = ptr.Of("none")
	expect.Mounts[0].Virtiofs.QueueSize = ptr.Of(2048)

	expect.MountType = ptr.Of(limatype.NINEP)
	expect.MountInotify = ptr.Of(true)

	// o.Networks[1] is overriding the dExpected.Networks[0].Lima entry for the "def0" interface
	expect.Networks = slices.Concat(dExpected.Networks, y.Networks, []limatype.Network{o.Networks[0]})
	expect.Networks[0].Lima = o.Networks[1].Lima

	// Only highest prio DNS are retained
	expect.DNS = slices.Clone(o.DNS)

	// ONE remains from filledDefaults.Env; the rest are set from o
	expect.Env["ONE"] = y.Env["ONE"]

	expect.Param["ONE"] = y.Param["ONE"]

	expect.CACertificates.RemoveDefaults = ptr.Of(true)
	expect.CACertificates.Files = []string{"ca.crt"}
	expect.CACertificates.Certs = []string{
		"-----BEGIN CERTIFICATE-----\nYOUR-ORGS-TRUSTED-CA-CERT\n-----END CERTIFICATE-----\n",
	}

	expect.Plain = ptr.Of(false)

	expect.NestedVirtualization = ptr.Of(false)

	expect.VMOpts = limatype.VMOpts{
		"qemu": dExpected.VMOpts["qemu"],
		"vz": map[string]any{
			"rosetta": map[string]any{
				"enabled": true,
				"binfmt":  true,
			},
		},
	}
	expect.TPM = ptr.Of(false)

	FillDefault(t.Context(), &y, &d, &o, filePath, false)
	assert.DeepEqual(t, &y, &expect, opts...)
}

func TestContainerdDefault(t *testing.T) {
	archives := defaultContainerdArchives()
	assert.Assert(t, len(archives) > 0)
}

func TestStaticPortForwarding(t *testing.T) {
	tests := []struct {
		name     string
		config   limatype.LimaYAML
		expected []limatype.PortForward
	}{
		{
			name: "plain mode with static port forwards",
			config: limatype.LimaYAML{
				Plain: ptr.Of(true),
				PortForwards: []limatype.PortForward{
					{
						GuestPort: 8080,
						HostPort:  8080,
						Static:    true,
					},
					{
						GuestPort: 9000,
						HostPort:  9000,
						Static:    false,
					},
					{
						GuestPort: 8081,
						HostPort:  8081,
					},
				},
			},
			expected: []limatype.PortForward{
				{
					GuestPort: 8080,
					HostPort:  8080,
					Static:    true,
				},
			},
		},
		{
			name: "non-plain mode with static port forwards",
			config: limatype.LimaYAML{
				Plain: ptr.Of(false),
				PortForwards: []limatype.PortForward{
					{
						GuestPort: 8080,
						HostPort:  8080,
						Static:    true,
					},
					{
						GuestPort: 9000,
						HostPort:  9000,
						Static:    false,
					},
				},
			},
			expected: []limatype.PortForward{
				{
					GuestPort: 8080,
					HostPort:  8080,
					Static:    true,
				},
				{
					GuestPort: 9000,
					HostPort:  9000,
					Static:    false,
				},
			},
		},
		{
			name: "plain mode with no static port forwards",
			config: limatype.LimaYAML{
				Plain: ptr.Of(true),
				PortForwards: []limatype.PortForward{
					{
						GuestPort: 8080,
						HostPort:  8080,
						Static:    false,
					},
					{
						GuestPort: 9000,
						HostPort:  9000,
					},
				},
			},
			expected: []limatype.PortForward{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fixUpForPlainMode(&tt.config)

			if *tt.config.Plain {
				for _, pf := range tt.config.PortForwards {
					if !pf.Static {
						t.Errorf("Non-static port forward found in plain mode: %+v", pf)
					}
				}
			}

			assert.Equal(t, len(tt.config.PortForwards), len(tt.expected),
				"Expected %d port forwards, got %d", len(tt.expected), len(tt.config.PortForwards))

			for i, expected := range tt.expected {
				if i >= len(tt.config.PortForwards) {
					t.Errorf("Missing port forward at index %d", i)
					continue
				}
				actual := tt.config.PortForwards[i]
				assert.Equal(t, expected.Static, actual.Static,
					"Port forward %d: expected Static=%v, got %v", i, expected.Static, actual.Static)
				assert.Equal(t, expected.GuestPort, actual.GuestPort,
					"Port forward %d: expected GuestPort=%d, got %d", i, expected.GuestPort, actual.GuestPort)
				assert.Equal(t, expected.HostPort, actual.HostPort,
					"Port forward %d: expected HostPort=%d, got %d", i, expected.HostPort, actual.HostPort)
			}
		})
	}
}

func TestMountTag(t *testing.T) {
	tests := []struct {
		name       string
		location   string
		mountPoint string
	}{
		{"home directory", "/Users/testuser", "/Users/testuser"},
		{"nested path", "/Users/testuser/Development/project", "/mnt/project"},
		{"root path", "/", "/mnt/root"},
		{"tmp directory", "/tmp/lima", "/tmp/lima"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tag := MountTag(tt.location, tt.mountPoint)
			assert.Assert(t, len(tag) > 5 && tag[:5] == "lima-")
			assert.Assert(t, len(tag) <= 36)
			assert.Equal(t, tag, MountTag(tt.location, tt.mountPoint))
		})
	}
}

func TestMountTagUniqueness(t *testing.T) {
	mounts := []struct{ location, mountPoint string }{
		{"/Users/user1", "/Users/user1"},
		{"/Users/user2", "/Users/user2"},
		{"/home/user1", "/home/user1"},
		{"/mnt/data", "/mnt/data"},
		{"/tmp/lima", "/tmp/lima"},
	}
	tags := make(map[string]bool)
	for _, m := range mounts {
		tag := MountTag(m.location, m.mountPoint)
		assert.Assert(t, !tags[tag], "tag collision: %s", tag)
		tags[tag] = true
	}
}

func TestMountTagDuplicateLocation(t *testing.T) {
	location := "/Users/testuser"
	tag1 := MountTag(location, "/home/user")
	tag2 := MountTag(location, "/mnt/home-writable")
	assert.Assert(t, tag1 != tag2)
}

// TestContainerdUserDefaultPerOS verifies that FillDefault only enables
// containerd.user=true on Linux guests for x86_64 and aarch64. nerdctl is a Linux-only runtime,
// so non-Linux guests (Darwin, FreeBSD) must default to false regardless
// of the host architecture. Regression test for issue #5037.
func TestContainerdUserDefaultPerOS(t *testing.T) {
	cases := []struct {
		os   limatype.OS
		arch limatype.Arch
		want bool
	}{
		{limatype.LINUX, limatype.X8664, true},
		{limatype.LINUX, limatype.AARCH64, true},
		{limatype.LINUX, limatype.RISCV64, false},
		{limatype.LINUX, limatype.ARMV7L, false},
		{limatype.DARWIN, limatype.AARCH64, false},
		{limatype.DARWIN, limatype.X8664, false},
		{limatype.FREEBSD, limatype.X8664, false},
		{limatype.FREEBSD, limatype.AARCH64, false},
	}
	for _, tc := range cases {
		t.Run(fmt.Sprintf("%s/%s", tc.os, tc.arch), func(t *testing.T) {
			y := limatype.LimaYAML{
				OS:   ptr.Of(tc.os),
				Arch: ptr.Of(tc.arch),
			}
			var d, o limatype.LimaYAML
			FillDefault(t.Context(), &y, &d, &o, "", false)
			assert.Assert(t, y.Containerd.User != nil, "Containerd.User must be set after FillDefault")
			assert.Equal(t, *y.Containerd.User, tc.want,
				"Containerd.User default for OS=%s Arch=%s", tc.os, tc.arch)
		})
	}
}

// TestContainerdUserExplicitOverride verifies FillDefault respects an
// explicit containerd.user=true value even on non-Linux guests.
func TestContainerdUserExplicitOverride(t *testing.T) {
	y := limatype.LimaYAML{
		OS:   ptr.Of(limatype.DARWIN),
		Arch: ptr.Of(limatype.AARCH64),
		Containerd: limatype.Containerd{
			User: ptr.Of(true),
		},
	}
	var d, o limatype.LimaYAML
	FillDefault(t.Context(), &y, &d, &o, "", false)
	assert.Assert(t, y.Containerd.User != nil)
	assert.Equal(t, *y.Containerd.User, true,
		"explicit Containerd.User=true must be preserved on non-Linux guests")
}

func TestUnmarshalImageVariant(t *testing.T) {
	yamlBytes := []byte(`
images:
- location: "server-img.img"
  variant: "server"
- location: "minimal-img.img"
  variant: "minimal"
`)
	var y limatype.LimaYAML
	err := Unmarshal(yamlBytes, &y, "test")
	assert.NilError(t, err)
	assert.Equal(t, len(y.Images), 2)
	assert.Equal(t, y.Images[0].Variant, "server")
	assert.Equal(t, y.Images[1].Variant, "minimal")
}

func TestDefaultSelectedImageIsStandard(t *testing.T) {
	// Verify that the default order of the images in the standard template
	// places standard/server before minimal so standard users don't regress.
	data, err := os.ReadFile("../../templates/_images/ubuntu-24.04.yaml")
	assert.NilError(t, err)
	var y limatype.LimaYAML
	err = Unmarshal(data, &y, "ubuntu-24.04.yaml")
	assert.NilError(t, err)
	var foundServer, foundMinimal bool
	for _, img := range y.Images {
		switch img.Variant {
		case "server":
			foundServer = true
			assert.Assert(t, !foundMinimal, "standard/server images must be listed before minimal images to prevent fallback regressions")
		case "minimal":
			foundMinimal = true
		}
	}
	assert.Assert(t, foundServer, "should have found standard/server images")
	assert.Assert(t, foundMinimal, "should have found minimal images")
}

func TestFillDefaultUserPasswordlessSudo(t *testing.T) {
	baseY := func() *limatype.LimaYAML {
		return &limatype.LimaYAML{
			OS:   ptr.Of(limatype.LINUX),
			Arch: ptr.Of(limatype.X8664),
		}
	}
	t.Run("fallback to true", func(t *testing.T) {
		y, d, o := baseY(), &limatype.LimaYAML{}, &limatype.LimaYAML{}
		FillDefault(t.Context(), y, d, o, "test.yaml", false)
		assert.Assert(t, y.User.PasswordlessSudo != nil)
		assert.Equal(t, *y.User.PasswordlessSudo, true)
	})
	t.Run("default propagates", func(t *testing.T) {
		y, d, o := baseY(), &limatype.LimaYAML{User: limatype.User{PasswordlessSudo: ptr.Of(false)}}, &limatype.LimaYAML{}
		FillDefault(t.Context(), y, d, o, "test.yaml", false)
		assert.Equal(t, *y.User.PasswordlessSudo, false)
	})
	t.Run("yaml wins over default", func(t *testing.T) {
		y, d, o := baseY(), &limatype.LimaYAML{User: limatype.User{PasswordlessSudo: ptr.Of(true)}}, &limatype.LimaYAML{}
		y.User.PasswordlessSudo = ptr.Of(false)
		FillDefault(t.Context(), y, d, o, "test.yaml", false)
		assert.Equal(t, *y.User.PasswordlessSudo, false)
	})
	t.Run("override wins", func(t *testing.T) {
		y, d, o := baseY(), &limatype.LimaYAML{}, &limatype.LimaYAML{User: limatype.User{PasswordlessSudo: ptr.Of(false)}}
		y.User.PasswordlessSudo = ptr.Of(true)
		FillDefault(t.Context(), y, d, o, "test.yaml", false)
		assert.Equal(t, *y.User.PasswordlessSudo, false)
	})
}

func TestValidatePasswordlessSudoRequiresPlain(t *testing.T) {
	y, d, o := &limatype.LimaYAML{}, &limatype.LimaYAML{}, &limatype.LimaYAML{}
	FillDefault(t.Context(), y, d, o, "test.yaml", false)
	y.Images = []limatype.Image{
		{
			File: limatype.File{
				Location: "https://example.com/dummy.img",
				Arch:     limatype.X8664,
			},
		},
	}
	y.User.PasswordlessSudo = ptr.Of(false)
	y.Plain = ptr.Of(false) // plain is false, which violates the GHSA constraint
	err := Validate(y, false)
	assert.ErrorContains(t, err, "`user.passwordlessSudo: false` requires `plain: true`")
	y.Plain = ptr.Of(true)
	errValid := Validate(y, false)
	assert.NilError(t, errValid)
}

func TestValidatePasswordlessSudoFalseNoPlainRequiredOnMacOS(t *testing.T) {
	y, d, o := &limatype.LimaYAML{}, &limatype.LimaYAML{}, &limatype.LimaYAML{}
	y.OS = ptr.Of(limatype.DARWIN)
	FillDefault(t.Context(), y, d, o, "test.yaml", false)
	y.Images = []limatype.Image{
		{File: limatype.File{Location: "https://example.com/dummy.img", Arch: limatype.AARCH64}},
	}
	err := Validate(y, false)
	assert.NilError(t, err)
}
