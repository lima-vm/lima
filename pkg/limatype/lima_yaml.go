// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package limatype

import (
	"net"
	"runtime"

	"github.com/lima-vm/go-qcow2reader/image"
	"github.com/opencontainers/go-digest"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/cpu"
)

type LimaYAML struct {
	Base               BaseTemplates `yaml:"base,omitempty" json:"base,omitempty"`
	MinimumLimaVersion *string       `yaml:"minimumLimaVersion,omitempty" json:"minimumLimaVersion,omitempty" jsonschema:"nullable"`
	VMType             *VMType       `yaml:"vmType,omitempty" json:"vmType,omitempty" jsonschema:"nullable"`
	VMOpts             VMOpts        `yaml:"vmOpts,omitempty" json:"vmOpts,omitempty"`
	OS                 *OS           `yaml:"os,omitempty" json:"os,omitempty" jsonschema:"nullable"`
	Arch               *Arch         `yaml:"arch,omitempty" json:"arch,omitempty" jsonschema:"nullable"`
	Images             []Image       `yaml:"images,omitempty" json:"images,omitempty" jsonschema:"nullable"`
	// Deprecated: Use vmOpts.qemu.cpuType instead.
	CPUType               CPUType       `yaml:"cpuType,omitempty" json:"cpuType,omitempty" jsonschema:"nullable"`
	CPUs                  *int          `yaml:"cpus,omitempty" json:"cpus,omitempty" jsonschema:"nullable"`
	Memory                *string       `yaml:"memory,omitempty" json:"memory,omitempty" jsonschema:"nullable"` // go-units.RAMInBytes
	Disk                  *string       `yaml:"disk,omitempty" json:"disk,omitempty" jsonschema:"nullable"`     // go-units.RAMInBytes
	AdditionalDisks       []Disk        `yaml:"additionalDisks,omitempty" json:"additionalDisks,omitempty" jsonschema:"nullable"`
	Mounts                []Mount       `yaml:"mounts,omitempty" json:"mounts,omitempty"`
	MountTypesUnsupported []string      `yaml:"mountTypesUnsupported,omitempty" json:"mountTypesUnsupported,omitempty" jsonschema:"nullable"`
	MountType             *MountType    `yaml:"mountType,omitempty" json:"mountType,omitempty" jsonschema:"nullable"`
	MountInotify          *bool         `yaml:"mountInotify,omitempty" json:"mountInotify,omitempty" jsonschema:"nullable"`
	SSH                   SSH           `yaml:"ssh,omitempty" json:"ssh,omitempty"` // REQUIRED (FIXME)
	Firmware              Firmware      `yaml:"firmware,omitempty" json:"firmware,omitempty"`
	Audio                 Audio         `yaml:"audio,omitempty" json:"audio,omitempty"`
	Video                 Video         `yaml:"video,omitempty" json:"video,omitempty"`
	Provision             []Provision   `yaml:"provision,omitempty" json:"provision,omitempty"`
	UpgradePackages       *bool         `yaml:"upgradePackages,omitempty" json:"upgradePackages,omitempty" jsonschema:"nullable"`
	Containerd            Containerd    `yaml:"containerd,omitempty" json:"containerd,omitempty"`
	GuestInstallPrefix    *string       `yaml:"guestInstallPrefix,omitempty" json:"guestInstallPrefix,omitempty" jsonschema:"nullable"`
	Probes                []Probe       `yaml:"probes,omitempty" json:"probes,omitempty"`
	PortForwards          []PortForward `yaml:"portForwards,omitempty" json:"portForwards,omitempty"`
	CopyToHost            []CopyToHost  `yaml:"copyToHost,omitempty" json:"copyToHost,omitempty"`
	Message               string        `yaml:"message,omitempty" json:"message,omitempty"`
	Networks              []Network     `yaml:"networks,omitempty" json:"networks,omitempty" jsonschema:"nullable"`
	// `network` was deprecated in Lima v0.7.0, removed in Lima v0.14.0. Use `networks` instead.
	Env          map[string]string `yaml:"env,omitempty" json:"env,omitempty"`
	Param        map[string]string `yaml:"param,omitempty" json:"param,omitempty"`
	DNS          []net.IP          `yaml:"dns,omitempty" json:"dns,omitempty"`
	HostResolver HostResolver      `yaml:"hostResolver,omitempty" json:"hostResolver,omitempty"`
	// `useHostResolver` was deprecated in Lima v0.8.1, removed in Lima v0.14.0. Use `hostResolver.enabled` instead.
	PropagateProxyEnv *bool          `yaml:"propagateProxyEnv,omitempty" json:"propagateProxyEnv,omitempty" jsonschema:"nullable"`
	CACertificates    CACertificates `yaml:"caCerts,omitempty" json:"caCerts,omitempty"`
	// Deprecated: Use vmOpts.vz.rosetta instead.
	Rosetta              Rosetta `yaml:"rosetta,omitempty" json:"rosetta,omitempty"`
	Plain                *bool   `yaml:"plain,omitempty" json:"plain,omitempty" jsonschema:"nullable"`
	TimeZone             *string `yaml:"timezone,omitempty" json:"timezone,omitempty" jsonschema:"nullable"`
	NestedVirtualization *bool   `yaml:"nestedVirtualization,omitempty" json:"nestedVirtualization,omitempty" jsonschema:"nullable"`
	User                 User    `yaml:"user,omitempty" json:"user,omitempty"`
}

type BaseTemplates []LocatorWithDigest

type LocatorWithDigest struct {
	URL    string  `yaml:"url" json:"url"`
	Digest *string `yaml:"digest,omitempty" json:"digest,omitempty"` // TODO currently unused
}

type (
	OS        = string
	Arch      = string
	MountType = string
	VMType    = string
)

type CPUType = map[Arch]string

const (
	LINUX   OS = "Linux"
	DARWIN  OS = "Darwin"
	FREEBSD OS = "FreeBSD"

	X8664   Arch = "x86_64"
	AARCH64 Arch = "aarch64"
	ARMV7L  Arch = "armv7l"
	PPC64LE Arch = "ppc64le"
	RISCV64 Arch = "riscv64"
	S390X   Arch = "s390x"

	REVSSHFS MountType = "reverse-sshfs"
	NINEP    MountType = "9p"
	VIRTIOFS MountType = "virtiofs"
	WSLMount MountType = "wsl2"

	QEMU VMType = "qemu"
	VZ   VMType = "vz"
	WSL2 VMType = "wsl2"
)

var (
	OSTypes    = []OS{LINUX, DARWIN, FREEBSD}
	ArchTypes  = []Arch{X8664, AARCH64, ARMV7L, PPC64LE, RISCV64, S390X}
	MountTypes = []MountType{REVSSHFS, NINEP, VIRTIOFS, WSLMount}
	VMTypes    = []VMType{QEMU, VZ, WSL2}
)

type User struct {
	Name    *string `yaml:"name,omitempty" json:"name,omitempty" jsonschema:"nullable"`
	Comment *string `yaml:"comment,omitempty" json:"comment,omitempty" jsonschema:"nullable"`
	Home    *string `yaml:"home,omitempty" json:"home,omitempty" jsonschema:"nullable"`
	Shell   *string `yaml:"shell,omitempty" json:"shell,omitempty" jsonschema:"nullable"`
	UID     *uint32 `yaml:"uid,omitempty" json:"uid,omitempty" jsonschema:"nullable"`
}

type VMOpts map[VMType]any

type QEMUOpts struct {
	MinimumVersion *string `yaml:"minimumVersion,omitempty" json:"minimumVersion,omitempty" jsonschema:"nullable"`
	CPUType        CPUType `yaml:"cpuType,omitempty" json:"cpuType,omitempty" jsonschema:"nullable"`
}

type VZOpts struct {
	Rosetta         Rosetta       `yaml:"rosetta,omitempty" json:"rosetta,omitempty"`
	DiskImageFormat *image.Type   `yaml:"diskImageFormat,omitempty" json:"diskImageFormat,omitempty" jsonschema:"nullable"`
	MemoryBalloon   MemoryBalloon `yaml:"memoryBalloon,omitempty" json:"memoryBalloon,omitempty"`
	AutoPause       AutoPause     `yaml:"autoPause,omitempty" json:"autoPause,omitempty"`
}

// MemoryBalloon configures dynamic memory ballooning for the VZ backend.
// When enabled, the balloon controller automatically shrinks guest memory
// when idle and grows it under pressure, returning unused RAM to the host.
// All fields are pointers to distinguish "not specified" (nil) from explicit values.
type MemoryBalloon struct {
	// Enabled enables/disables memory ballooning.
	Enabled *bool `yaml:"enabled,omitempty" json:"enabled,omitempty" jsonschema:"nullable"`
	// Min is the minimum guest memory size (e.g., "3GiB"). The balloon will never shrink below this.
	Min *string `yaml:"min,omitempty" json:"min,omitempty" jsonschema:"nullable"`
	// IdleTarget is the target memory when the VM is idle (e.g., "4GiB"). Must be > Min and <= Memory.
	IdleTarget *string `yaml:"idleTarget,omitempty" json:"idleTarget,omitempty" jsonschema:"nullable"`
	// GrowStepPercent is the percentage of max memory to grow per step (1-100).
	GrowStepPercent *int `yaml:"growStepPercent,omitempty" json:"growStepPercent,omitempty" jsonschema:"nullable"`
	// ShrinkStepPercent is the percentage of max memory to shrink per step (1-100).
	ShrinkStepPercent *int `yaml:"shrinkStepPercent,omitempty" json:"shrinkStepPercent,omitempty" jsonschema:"nullable"`
	// HighPressureThreshold is the PSI some10 value that triggers growth (0.0-1.0).
	HighPressureThreshold *float64 `yaml:"highPressureThreshold,omitempty" json:"highPressureThreshold,omitempty" jsonschema:"nullable"`
	// LowPressureThreshold is the PSI some10 value below which shrinking is allowed (0.0-1.0).
	LowPressureThreshold *float64 `yaml:"lowPressureThreshold,omitempty" json:"lowPressureThreshold,omitempty" jsonschema:"nullable"`
	// Cooldown is the minimum time between balloon actions (e.g., "30s").
	Cooldown *string `yaml:"cooldown,omitempty" json:"cooldown,omitempty" jsonschema:"nullable"`
	// IdleGracePeriod is how long after boot before ballooning begins (e.g., "5m").
	IdleGracePeriod *string `yaml:"idleGracePeriod,omitempty" json:"idleGracePeriod,omitempty" jsonschema:"nullable"`
	// MaxSwapInPerSec is the swap-in rate threshold that blocks shrinking (e.g., "50MiB").
	MaxSwapInPerSec *string `yaml:"maxSwapInPerSec,omitempty" json:"maxSwapInPerSec,omitempty" jsonschema:"nullable"`
	// MaxContainerCPU is the container CPU usage threshold that blocks shrinking (percentage, e.g., 10.0).
	MaxContainerCPU *float64 `yaml:"maxContainerCPU,omitempty" json:"maxContainerCPU,omitempty" jsonschema:"nullable"`
	// MaxContainerIO is the container I/O rate threshold that blocks shrinking (e.g., "100MiB").
	MaxContainerIO *string `yaml:"maxContainerIO,omitempty" json:"maxContainerIO,omitempty" jsonschema:"nullable"`
}

// AutoPause configures automatic VM pausing when idle for the VZ backend.
// When enabled, the VM is paused after a period of inactivity and resumed
// transparently on user activity (shell access, socket connection).
// All fields are pointers to distinguish "not specified" (nil) from explicit values.
type AutoPause struct {
	// Enabled enables/disables auto-pause.
	Enabled *bool `yaml:"enabled,omitempty" json:"enabled,omitempty" jsonschema:"nullable"`
	// IdleTimeout is how long the VM must be idle before pausing (e.g., "15m"). Minimum 1m.
	IdleTimeout *string `yaml:"idleTimeout,omitempty" json:"idleTimeout,omitempty" jsonschema:"nullable"`
	// ResumeTimeout is the maximum time to wait for a resume operation (e.g., "30s"). Minimum 5s.
	ResumeTimeout *string `yaml:"resumeTimeout,omitempty" json:"resumeTimeout,omitempty" jsonschema:"nullable"`
	// IdleSignals configures which activity signals prevent auto-pause.
	IdleSignals IdleSignals `yaml:"idleSignals,omitempty" json:"idleSignals,omitempty"`
}

// IdleSignals configures which activity signals prevent VM auto-pause.
// All signals default to enabled (true) when not specified.
type IdleSignals struct {
	// ActiveConnections tracks open proxy socket connections as VM activity.
	// When true, any active bicopy relay session prevents pause.
	ActiveConnections *bool `yaml:"activeConnections,omitempty" json:"activeConnections,omitempty" jsonschema:"nullable"`
	// ContainerCPU tracks container CPU usage as VM activity.
	// When true, containers with CPU above ContainerCPUThreshold prevent pause.
	ContainerCPU *bool `yaml:"containerCPU,omitempty" json:"containerCPU,omitempty" jsonschema:"nullable"`
	// ContainerCPUThreshold is the minimum CPU percentage to consider containers active.
	// Only used when ContainerCPU is enabled. Default: 0.5 (0.5%). Range: 0.0–100.0.
	ContainerCPUThreshold *float64 `yaml:"containerCPUThreshold,omitempty" json:"containerCPUThreshold,omitempty" jsonschema:"nullable"`
	// ContainerIO tracks container IO byte rate changes as VM activity.
	// When true, changing IO rates prevent pause.
	ContainerIO *bool `yaml:"containerIO,omitempty" json:"containerIO,omitempty" jsonschema:"nullable"`
}

type Rosetta struct {
	Enabled *bool `yaml:"enabled,omitempty" json:"enabled,omitempty" jsonschema:"nullable"`
	BinFmt  *bool `yaml:"binfmt,omitempty" json:"binfmt,omitempty" jsonschema:"nullable"`
}

type File struct {
	Location string        `yaml:"location" json:"location"` // REQUIRED
	Arch     Arch          `yaml:"arch,omitempty" json:"arch,omitempty"`
	Digest   digest.Digest `yaml:"digest,omitempty" json:"digest,omitempty"`
}

type FileWithVMType struct {
	File   `yaml:",inline"`
	VMType VMType `yaml:"vmType,omitempty" json:"vmType,omitempty"`
}

type Kernel struct {
	File    `yaml:",inline"`
	Cmdline string `yaml:"cmdline,omitempty" json:"cmdline,omitempty"`
}

type Image struct {
	File   `yaml:",inline"`
	Kernel *Kernel `yaml:"kernel,omitempty" json:"kernel,omitempty"`
	Initrd *File   `yaml:"initrd,omitempty" json:"initrd,omitempty"`
}

type Disk struct {
	Name   string   `yaml:"name" json:"name"` // REQUIRED
	Format *bool    `yaml:"format,omitempty" json:"format,omitempty"`
	FSType *string  `yaml:"fsType,omitempty" json:"fsType,omitempty"`
	FSArgs []string `yaml:"fsArgs,omitempty" json:"fsArgs,omitempty"`
}

type Mount struct {
	Location   string   `yaml:"location" json:"location"` // REQUIRED
	MountPoint *string  `yaml:"mountPoint,omitempty" json:"mountPoint,omitempty" jsonschema:"nullable"`
	Writable   *bool    `yaml:"writable,omitempty" json:"writable,omitempty" jsonschema:"nullable"`
	SSHFS      SSHFS    `yaml:"sshfs,omitempty" json:"sshfs,omitempty"`
	NineP      NineP    `yaml:"9p,omitempty" json:"9p,omitempty"`
	Virtiofs   Virtiofs `yaml:"virtiofs,omitempty" json:"virtiofs,omitempty"`
}

type SFTPDriver = string

const (
	SFTPDriverBuiltin           = "builtin"
	SFTPDriverOpenSSHSFTPServer = "openssh-sftp-server"
)

type SSHFS struct {
	Cache          *bool       `yaml:"cache,omitempty" json:"cache,omitempty" jsonschema:"nullable"`
	FollowSymlinks *bool       `yaml:"followSymlinks,omitempty" json:"followSymlinks,omitempty" jsonschema:"nullable"`
	SFTPDriver     *SFTPDriver `yaml:"sftpDriver,omitempty" json:"sftpDriver,omitempty" jsonschema:"nullable"`
}

type NineP struct {
	SecurityModel   *string `yaml:"securityModel,omitempty" json:"securityModel,omitempty" jsonschema:"nullable"`
	ProtocolVersion *string `yaml:"protocolVersion,omitempty" json:"protocolVersion,omitempty" jsonschema:"nullable"`
	Msize           *string `yaml:"msize,omitempty" json:"msize,omitempty" jsonschema:"nullable"`
	Cache           *string `yaml:"cache,omitempty" json:"cache,omitempty" jsonschema:"nullable"`
}

type Virtiofs struct {
	QueueSize *int `yaml:"queueSize,omitempty" json:"queueSize,omitempty"`
}

type SSH struct {
	LocalPort *int `yaml:"localPort,omitempty" json:"localPort,omitempty" jsonschema:"nullable"`

	// LoadDotSSHPubKeys loads ~/.ssh/*.pub in addition to $LIMA_HOME/_config/user.pub .
	LoadDotSSHPubKeys *bool `yaml:"loadDotSSHPubKeys,omitempty" json:"loadDotSSHPubKeys,omitempty" jsonschema:"nullable"` // default: false
	ForwardAgent      *bool `yaml:"forwardAgent,omitempty" json:"forwardAgent,omitempty" jsonschema:"nullable"`           // default: false
	ForwardX11        *bool `yaml:"forwardX11,omitempty" json:"forwardX11,omitempty" jsonschema:"nullable"`               // default: false
	ForwardX11Trusted *bool `yaml:"forwardX11Trusted,omitempty" json:"forwardX11Trusted,omitempty" jsonschema:"nullable"` // default: false

	OverVsock *bool `yaml:"overVsock,omitempty" json:"overVsock,omitempty" jsonschema:"nullable"` // default: depends on VMType
}

type Firmware struct {
	// LegacyBIOS disables UEFI if set.
	// LegacyBIOS is ignored for aarch64.
	LegacyBIOS *bool `yaml:"legacyBIOS,omitempty" json:"legacyBIOS,omitempty" jsonschema:"nullable"`

	// Images specify UEFI images (edk2-aarch64-code.fd.gz).
	// Defaults to built-in UEFI.
	Images []FileWithVMType `yaml:"images,omitempty" json:"images,omitempty"`
}

type Audio struct {
	// Device is a QEMU audiodev string
	Device *string `yaml:"device,omitempty" json:"device,omitempty" jsonschema:"nullable"`
}

type VNCOptions struct {
	Display *string `yaml:"display,omitempty" json:"display,omitempty" jsonschema:"nullable"`
}

type Video struct {
	// Display is a QEMU display string
	Display *string    `yaml:"display,omitempty" json:"display,omitempty" jsonschema:"nullable"`
	VNC     VNCOptions `yaml:"vnc,omitempty" json:"vnc,omitempty"`
}

type ProvisionMode = string

const (
	ProvisionModeSystem     ProvisionMode = "system"
	ProvisionModeUser       ProvisionMode = "user"
	ProvisionModeBoot       ProvisionMode = "boot"
	ProvisionModeDependency ProvisionMode = "dependency"
	ProvisionModeAnsible    ProvisionMode = "ansible" // DEPRECATED
	ProvisionModeData       ProvisionMode = "data"
	ProvisionModeYQ         ProvisionMode = "yq"
)

type Provision struct {
	Mode                            ProvisionMode      `yaml:"mode,omitempty" json:"mode,omitempty" jsonschema:"default=system"`
	SkipDefaultDependencyResolution *bool              `yaml:"skipDefaultDependencyResolution,omitempty" json:"skipDefaultDependencyResolution,omitempty"`
	Script                          *string            `yaml:"script,omitempty" json:"script,omitempty"`
	File                            *LocatorWithDigest `yaml:"file,omitempty" json:"file,omitempty" jsonschema:"nullable"`
	Playbook                        string             `yaml:"playbook,omitempty" json:"playbook,omitempty"` // DEPRECATED
	// All ProvisionData fields must be nil unless Mode is ProvisionModeData
	ProvisionData `yaml:",inline"` // Flatten fields for "strict" YAML mode
	// ProvisionModeYQ borrows Owner, Path, and Permissions from ProvisionData
	Expression *string `yaml:"expression,omitempty" json:"expression,omitempty" jsonschema:"nullable"`
	Format     *string `yaml:"format,omitempty" json:"format,omitempty" jsonschema:"nullable"`
}

type ProvisionData struct {
	Content     *string `yaml:"content,omitempty" json:"content,omitempty" jsonschema:"nullable"`
	Overwrite   *bool   `yaml:"overwrite,omitempty" json:"overwrite,omitempty" jsonschema:"nullable"`
	Owner       *string `yaml:"owner,omitempty" json:"owner,omitempty"` // any owner string supported by `chown`, defaults to "root:root" on Linux
	Path        *string `yaml:"path,omitempty" json:"path,omitempty"`
	Permissions *string `yaml:"permissions,omitempty" json:"permissions,omitempty"`
}

type Containerd struct {
	System   *bool  `yaml:"system,omitempty" json:"system,omitempty" jsonschema:"nullable"` // default: false
	User     *bool  `yaml:"user,omitempty" json:"user,omitempty" jsonschema:"nullable"`     // default: true
	Archives []File `yaml:"archives,omitempty" json:"archives,omitempty"`                   // default: see defaultContainerdArchives
}

type ProbeMode = string

const (
	ProbeModeReadiness ProbeMode = "readiness"
)

type Probe struct {
	Mode        ProbeMode          `yaml:"mode,omitempty" json:"mode,omitempty" jsonschema:"default=readiness"`
	Description string             `yaml:"description,omitempty" json:"description,omitempty"`
	Script      *string            `yaml:"script,omitempty" json:"script,omitempty"`
	File        *LocatorWithDigest `yaml:"file,omitempty" json:"file,omitempty" jsonschema:"nullable"`
	Hint        string             `yaml:"hint,omitempty" json:"hint,omitempty"`
}

type Proto = string

const (
	ProtoTCP Proto = "tcp"
	ProtoUDP Proto = "udp"
	ProtoAny Proto = "any"
)

type PortForward struct {
	GuestIPMustBeZero *bool  `yaml:"guestIPMustBeZero,omitempty" json:"guestIPMustBeZero,omitempty"`
	GuestIP           net.IP `yaml:"guestIP,omitempty" json:"guestIP,omitempty"`
	GuestPort         int    `yaml:"guestPort,omitempty" json:"guestPort,omitempty"`
	GuestPortRange    [2]int `yaml:"guestPortRange,omitempty" json:"guestPortRange,omitempty"`
	GuestSocket       string `yaml:"guestSocket,omitempty" json:"guestSocket,omitempty"`
	HostIP            net.IP `yaml:"hostIP,omitempty" json:"hostIP,omitempty"`
	HostPort          int    `yaml:"hostPort,omitempty" json:"hostPort,omitempty"`
	HostPortRange     [2]int `yaml:"hostPortRange,omitempty" json:"hostPortRange,omitempty"`
	HostSocket        string `yaml:"hostSocket,omitempty" json:"hostSocket,omitempty"`
	Proto             Proto  `yaml:"proto,omitempty" json:"proto,omitempty"`
	Reverse           bool   `yaml:"reverse,omitempty" json:"reverse,omitempty"`
	Ignore            bool   `yaml:"ignore,omitempty" json:"ignore,omitempty"`
	Static            bool   `yaml:"static,omitempty" json:"static,omitempty"`
}

type CopyToHost struct {
	GuestFile    string `yaml:"guest,omitempty" json:"guest,omitempty"`
	HostFile     string `yaml:"host,omitempty" json:"host,omitempty"`
	DeleteOnStop bool   `yaml:"deleteOnStop,omitempty" json:"deleteOnStop,omitempty"`
}

type Network struct {
	// `Lima` and `Socket` are mutually exclusive; exactly one is required
	Lima string `yaml:"lima,omitempty" json:"lima,omitempty"`
	// Socket is a QEMU-compatible socket
	Socket string `yaml:"socket,omitempty" json:"socket,omitempty"`
	// VZNAT uses VZNATNetworkDeviceAttachment. Needs VZ. No root privilege is required.
	VZNAT *bool `yaml:"vzNAT,omitempty" json:"vzNAT,omitempty"`

	MACAddress string  `yaml:"macAddress,omitempty" json:"macAddress,omitempty"`
	Interface  string  `yaml:"interface,omitempty" json:"interface,omitempty"`
	Metric     *uint32 `yaml:"metric,omitempty" json:"metric,omitempty"`
}

type HostResolver struct {
	Enabled *bool             `yaml:"enabled,omitempty" json:"enabled,omitempty" jsonschema:"nullable"`
	IPv6    *bool             `yaml:"ipv6,omitempty" json:"ipv6,omitempty" jsonschema:"nullable"`
	Hosts   map[string]string `yaml:"hosts,omitempty" json:"hosts,omitempty" jsonschema:"nullable"`
}

type CACertificates struct {
	RemoveDefaults *bool    `yaml:"removeDefaults,omitempty" json:"removeDefaults,omitempty" jsonschema:"nullable"` // default: false
	Files          []string `yaml:"files,omitempty" json:"files,omitempty" jsonschema:"nullable"`
	Certs          []string `yaml:"certs,omitempty" json:"certs,omitempty" jsonschema:"nullable"`
}

type PreConfiguredDriverPayload struct {
	Config   LimaYAML `json:"config"`
	FilePath string   `json:"filePath"`
}

func NewOS(osname string) OS {
	switch osname {
	case "linux":
		return LINUX
	case "darwin":
		return DARWIN
	default:
		logrus.Warnf("Unknown os: %s", osname)
		return osname
	}
}

func Goarm() int {
	if runtime.GOOS != "linux" {
		return 0
	}
	if runtime.GOARCH != "arm" {
		return 0
	}
	if cpu.ARM.HasVFPv3 {
		return 7
	}
	if cpu.ARM.HasVFP {
		return 6
	}
	return 5 // default
}

func NewArch(arch string) Arch {
	switch arch {
	case "amd64":
		return X8664
	case "arm64":
		return AARCH64
	case "arm":
		arm := Goarm()
		if arm == 7 {
			return ARMV7L
		}
		logrus.Warnf("Unknown arm: %d", arm)
		return arch
	case "ppc64le":
		return PPC64LE
	case "riscv64":
		return RISCV64
	case "s390x":
		return S390X
	default:
		logrus.Warnf("Unknown arch: %s", arch)
		return arch
	}
}

func IsNativeArch(arch Arch) bool {
	nativeX8664 := arch == X8664 && runtime.GOARCH == "amd64"
	nativeAARCH64 := arch == AARCH64 && runtime.GOARCH == "arm64"
	nativeARMV7L := arch == ARMV7L && runtime.GOARCH == "arm" && Goarm() == 7
	nativePPC64LE := arch == PPC64LE && runtime.GOARCH == "ppc64le"
	nativeRISCV64 := arch == RISCV64 && runtime.GOARCH == "riscv64"
	nativeS390X := arch == S390X && runtime.GOARCH == "s390x"
	return nativeX8664 || nativeAARCH64 || nativeARMV7L || nativePPC64LE || nativeRISCV64 || nativeS390X
}

func DefaultDriver() VMType {
	switch runtime.GOOS {
	case "darwin":
		return VZ
	case "windows":
		return WSL2
	default:
		return QEMU
	}
}

func DefaultNonNativeArchDriver() VMType {
	return QEMU
}
