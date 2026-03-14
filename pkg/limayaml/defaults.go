// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package limayaml

import (
	"bytes"
	"context"
	"crypto/sha256"
	_ "embed"
	"errors"
	"fmt"
	"maps"
	"net"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"text/template"

	"github.com/docker/go-units"
	"github.com/goccy/go-yaml"
	"github.com/pbnjay/memory"
	"github.com/sirupsen/logrus"

	"github.com/lima-vm/lima/v2/pkg/instance/hostname"
	"github.com/lima-vm/lima/v2/pkg/ioutilx"
	"github.com/lima-vm/lima/v2/pkg/limatype"
	"github.com/lima-vm/lima/v2/pkg/limatype/dirnames"
	"github.com/lima-vm/lima/v2/pkg/limatype/filenames"
	"github.com/lima-vm/lima/v2/pkg/localpathutil"
	. "github.com/lima-vm/lima/v2/pkg/must"
	"github.com/lima-vm/lima/v2/pkg/networks"
	"github.com/lima-vm/lima/v2/pkg/osutil"
	"github.com/lima-vm/lima/v2/pkg/ptr"
	"github.com/lima-vm/lima/v2/pkg/version"
)

const (
	// Default9pSecurityModel is "none" for supporting symlinks
	// https://gitlab.com/qemu-project/qemu/-/issues/173
	Default9pSecurityModel   string = "none"
	Default9pProtocolVersion string = "9p2000.L"
	Default9pMsize           string = "128KiB"
	Default9pCacheForRO      string = "fscache"
	Default9pCacheForRW      string = "mmap"

	DefaultVirtiofsQueueSize int = 1024
)

var (
	IPv4loopback1 = net.IPv4(127, 0, 0, 1)

	userHomeDir = Must(os.UserHomeDir())
	currentUser = Must(user.Current())
)

//go:embed containerd.yaml
var defaultContainerdYAML []byte

type ContainerdYAML struct {
	Archives []limatype.File
}

func defaultContainerdArchives() []limatype.File {
	var containerd ContainerdYAML
	err := yaml.UnmarshalWithOptions(defaultContainerdYAML, &containerd, yaml.Strict())
	if err != nil {
		panic(fmt.Errorf("failed to unmarshal as YAML: %w", err))
	}
	return containerd.Archives
}

// FirstUsernetIndex gets the index of first usernet network under l.Network[]. Returns -1 if no usernet network found.
func FirstUsernetIndex(l *limatype.LimaYAML) int {
	return slices.IndexFunc(l.Networks, func(network limatype.Network) bool { return networks.IsUsernet(network.Lima) })
}

func MACAddress(uniqueID string) string {
	sha := sha256.Sum256([]byte(osutil.MachineID() + uniqueID))
	// "5" is the magic number in the Lima ecosystem.
	// (Visit https://en.wiktionary.org/wiki/lima and Command-F "five")
	//
	// But the second hex number is changed to 2 to satisfy the convention for
	// local MAC addresses (https://en.wikipedia.org/wiki/MAC_address#Ranges_of_group_and_locally_administered_addresses)
	//
	// See also https://gitlab.com/wireshark/wireshark/-/blob/release-4.0/manuf to confirm the uniqueness of this prefix.
	hw := append(net.HardwareAddr{0x52, 0x55, 0x55}, sha[0:3]...)
	return hw.String()
}

// MountTag generates a stable mount tag from location and mountPoint.
// Both paths are hashed to handle the same location mounted to multiple mount points.
func MountTag(location, mountPoint string) string {
	sha := sha256.Sum256([]byte(location + "\x00" + mountPoint))
	return fmt.Sprintf("lima-%x", sha[0:8])
}

func defaultCPUs() int {
	const x = 4
	if hostCPUs := runtime.NumCPU(); hostCPUs < x {
		return hostCPUs
	}
	return x
}

func defaultMemory() uint64 {
	const x uint64 = 4 * 1024 * 1024 * 1024
	if halfOfHostMemory := memory.TotalMemory() / 2; halfOfHostMemory < x {
		return halfOfHostMemory
	}
	return x
}

func defaultMemoryAsString() string {
	return units.BytesSize(float64(defaultMemory()))
}

func defaultDiskSizeAsString() string {
	// currently just hardcoded
	return "100GiB"
}

func defaultGuestInstallPrefix() string {
	return "/usr/local"
}

// FillDefault updates undefined fields in y with defaults from d (or built-in default), and overwrites with values from o.
// Both d and o may be empty.
//
// Maps (`Env`) are being merged: first populated from d, overwritten by y, and again overwritten by o.
// Slices (e.g. `Mounts`, `Provision`) are appended, starting with o, followed by y, and finally d. This
// makes sure o takes priority over y over d, in cases it matters (e.g. `PortForwards`, where the first
// matching rule terminates the search).
//
// Exceptions:
//   - Mounts are appended in d, y, o order, but "merged" when the Location matches a previous entry;
//     the highest priority Writable setting wins.
//   - Networks are appended in d, y, o order
//   - DNS are picked from the highest priority where DNS is not empty.
//   - CACertificates Files and Certs are uniquely appended in d, y, o order
func FillDefault(ctx context.Context, y, d, o *limatype.LimaYAML, filePath string, warn bool) {
	instDir := filepath.Dir(filePath)

	existingLimaVersion := ExistingLimaVersion(instDir)

	// OS has to be resolved before User
	if y.OS == nil {
		y.OS = d.OS
	}
	if o.OS != nil {
		y.OS = o.OS
	}
	y.OS = ptr.Of(ResolveOS(y.OS))

	if y.User.Name == nil {
		y.User.Name = d.User.Name
	}
	if y.User.Comment == nil {
		y.User.Comment = d.User.Comment
	}
	if y.User.Home == nil {
		y.User.Home = d.User.Home
	}
	if y.User.Shell == nil {
		y.User.Shell = d.User.Shell
	}
	if y.User.UID == nil {
		y.User.UID = d.User.UID
	}
	if o.User.Name != nil {
		y.User.Name = o.User.Name
	}
	if o.User.Comment != nil {
		y.User.Comment = o.User.Comment
	}
	if o.User.Home != nil {
		y.User.Home = o.User.Home
	}
	if o.User.Shell != nil {
		y.User.Shell = o.User.Shell
	}
	if o.User.UID != nil {
		y.User.UID = o.User.UID
	}
	if y.User.Name == nil {
		y.User.Name = ptr.Of(osutil.LimaUser(ctx, existingLimaVersion, warn, y.OS).Username)
		warn = false
	}
	if y.User.Comment == nil {
		y.User.Comment = ptr.Of(osutil.LimaUser(ctx, existingLimaVersion, warn, y.OS).Name)
		warn = false
	}
	if y.User.Home == nil {
		y.User.Home = ptr.Of(osutil.LimaUser(ctx, existingLimaVersion, warn, y.OS).HomeDir)
		warn = false
	}
	if y.User.Shell == nil {
		switch *y.OS {
		case limatype.FREEBSD:
			y.User.Shell = ptr.Of("/bin/sh")
		case limatype.DARWIN:
			y.User.Shell = ptr.Of("/bin/zsh")
		default:
			y.User.Shell = ptr.Of("/bin/bash")
		}
	}
	if y.User.UID == nil {
		uidString := osutil.LimaUser(ctx, existingLimaVersion, warn, y.OS).Uid
		if uid, err := strconv.ParseUint(uidString, 10, 32); err == nil {
			y.User.UID = ptr.Of(uint32(uid))
		} else {
			// This should never happen; LimaUser() makes sure that .Uid is numeric
			logrus.WithError(err).Warnf("Can't parse `user.uid` %q", uidString)
			y.User.UID = ptr.Of(uint32(1000))
		}
		// warn = false
	}
	if out, err := executeGuestTemplate(*y.User.Home, instDir, y.User, y.Param); err == nil {
		y.User.Home = ptr.Of(out.String())
	} else {
		logrus.WithError(err).Warnf("Couldn't process `user.home` value %q as a template", *y.User.Home)
	}

	if y.VMType == nil {
		y.VMType = d.VMType
	}
	if o.VMType != nil {
		y.VMType = o.VMType
	}

	if y.Arch == nil {
		y.Arch = d.Arch
	}
	if o.Arch != nil {
		y.Arch = o.Arch
	}
	y.Arch = ptr.Of(ResolveArch(y.Arch))

	y.Images = slices.Concat(o.Images, y.Images, d.Images)
	for i := range y.Images {
		img := &y.Images[i]
		if img.Arch == "" {
			img.Arch = *y.Arch
		}
		if img.Kernel != nil && img.Kernel.Arch == "" {
			img.Kernel.Arch = img.Arch
		}
		if img.Initrd != nil && img.Initrd.Arch == "" {
			img.Initrd.Arch = img.Arch
		}
	}

	if y.CPUs == nil {
		y.CPUs = d.CPUs
	}
	if o.CPUs != nil {
		y.CPUs = o.CPUs
	}
	if y.CPUs == nil || *y.CPUs == 0 {
		y.CPUs = ptr.Of(defaultCPUs())
	}

	if y.Memory == nil {
		y.Memory = d.Memory
	}
	if o.Memory != nil {
		y.Memory = o.Memory
	}
	if y.Memory == nil || *y.Memory == "" {
		y.Memory = ptr.Of(defaultMemoryAsString())
	}

	if y.Disk == nil {
		y.Disk = d.Disk
	}
	if o.Disk != nil {
		y.Disk = o.Disk
	}
	if y.Disk == nil || *y.Disk == "" {
		y.Disk = ptr.Of(defaultDiskSizeAsString())
	}

	y.AdditionalDisks = slices.Concat(o.AdditionalDisks, y.AdditionalDisks, d.AdditionalDisks)

	if y.Audio.Device == nil {
		y.Audio.Device = d.Audio.Device
	}
	if o.Audio.Device != nil {
		y.Audio.Device = o.Audio.Device
	}
	if y.Audio.Device == nil {
		y.Audio.Device = ptr.Of("")
	}

	if y.Video.Display == nil {
		y.Video.Display = d.Video.Display
	}
	if o.Video.Display != nil {
		y.Video.Display = o.Video.Display
	}
	if y.Video.Display == nil || *y.Video.Display == "" {
		y.Video.Display = ptr.Of("none")
	}

	if y.Video.VNC.Display == nil {
		y.Video.VNC.Display = d.Video.VNC.Display
	}
	if o.Video.VNC.Display != nil {
		y.Video.VNC.Display = o.Video.VNC.Display
	}

	if y.Firmware.LegacyBIOS == nil {
		y.Firmware.LegacyBIOS = d.Firmware.LegacyBIOS
	}
	if o.Firmware.LegacyBIOS != nil {
		y.Firmware.LegacyBIOS = o.Firmware.LegacyBIOS
	}
	if y.Firmware.LegacyBIOS == nil {
		y.Firmware.LegacyBIOS = ptr.Of(false)
	}

	y.Firmware.Images = slices.Concat(o.Firmware.Images, y.Firmware.Images, d.Firmware.Images)
	for i := range y.Firmware.Images {
		f := &y.Firmware.Images[i]
		if f.Arch == "" {
			f.Arch = *y.Arch
		}
	}

	if y.TimeZone == nil {
		y.TimeZone = d.TimeZone
	}
	if o.TimeZone != nil {
		y.TimeZone = o.TimeZone
	}
	if y.TimeZone == nil {
		y.TimeZone = ptr.Of(hostTimeZone())
	}

	if y.SSH.LocalPort == nil {
		y.SSH.LocalPort = d.SSH.LocalPort
	}
	if o.SSH.LocalPort != nil {
		y.SSH.LocalPort = o.SSH.LocalPort
	}
	if y.SSH.LocalPort == nil {
		// y.SSH.LocalPort value is not filled here (filled by the hostagent)
		y.SSH.LocalPort = ptr.Of(0)
	}
	if y.SSH.LoadDotSSHPubKeys == nil {
		y.SSH.LoadDotSSHPubKeys = d.SSH.LoadDotSSHPubKeys
	}
	if o.SSH.LoadDotSSHPubKeys != nil {
		y.SSH.LoadDotSSHPubKeys = o.SSH.LoadDotSSHPubKeys
	}
	if y.SSH.LoadDotSSHPubKeys == nil {
		y.SSH.LoadDotSSHPubKeys = ptr.Of(false) // was true before Lima v1.0
	}

	if y.SSH.ForwardAgent == nil {
		y.SSH.ForwardAgent = d.SSH.ForwardAgent
	}
	if o.SSH.ForwardAgent != nil {
		y.SSH.ForwardAgent = o.SSH.ForwardAgent
	}
	if y.SSH.ForwardAgent == nil {
		y.SSH.ForwardAgent = ptr.Of(false)
	}

	if y.SSH.ForwardX11 == nil {
		y.SSH.ForwardX11 = d.SSH.ForwardX11
	}
	if o.SSH.ForwardX11 != nil {
		y.SSH.ForwardX11 = o.SSH.ForwardX11
	}
	if y.SSH.ForwardX11 == nil {
		y.SSH.ForwardX11 = ptr.Of(false)
	}

	if y.SSH.ForwardX11Trusted == nil {
		y.SSH.ForwardX11Trusted = d.SSH.ForwardX11Trusted
	}
	if o.SSH.ForwardX11Trusted != nil {
		y.SSH.ForwardX11Trusted = o.SSH.ForwardX11Trusted
	}
	if y.SSH.ForwardX11Trusted == nil {
		y.SSH.ForwardX11Trusted = ptr.Of(false)
	}

	if y.SSH.OverVsock == nil {
		y.SSH.OverVsock = d.SSH.OverVsock
	}
	if o.SSH.OverVsock != nil {
		y.SSH.OverVsock = o.SSH.OverVsock
	}
	// y.SSH.OverVsock default value depends on the driver; filled in driver-specific FillDefault()

	// The deprecated environment variable LIMA_SSH_OVER_VSOCK takes precedence over .ssh.overVsock
	if envVar := os.Getenv("LIMA_SSH_OVER_VSOCK"); envVar != "" {
		logrus.Warn("The environment variable LIMA_SSH_OVER_VSOCK is deprecated in favor of the YAML field .ssh.overVsock")
		b, err := strconv.ParseBool(envVar)
		if err != nil {
			logrus.WithError(err).Warnf("invalid LIMA_SSH_OVER_VSOCK value %q", envVar)
		} else {
			logrus.Debugf("Overriding ssh.overVsock from %v to %v via LIMA_SSH_OVER_VSOCK", y.SSH.OverVsock, &b)
			y.SSH.OverVsock = ptr.Of(b)
		}
	}

	hosts := make(map[string]string)
	// Values can be either names or IP addresses. Name values are canonicalized in the hostResolver.
	maps.Copy(hosts, d.HostResolver.Hosts)
	maps.Copy(hosts, y.HostResolver.Hosts)
	maps.Copy(hosts, o.HostResolver.Hosts)
	y.HostResolver.Hosts = hosts

	y.Provision = slices.Concat(o.Provision, y.Provision, d.Provision)
	for i := range y.Provision {
		provision := &y.Provision[i]
		if provision.Mode == "" {
			provision.Mode = limatype.ProvisionModeSystem
		}
		if provision.Mode == limatype.ProvisionModeDependency && provision.SkipDefaultDependencyResolution == nil {
			provision.SkipDefaultDependencyResolution = ptr.Of(false)
		}
		if provision.Mode == limatype.ProvisionModeData {
			if provision.Content == nil {
				provision.Content = ptr.Of("")
			} else {
				if out, err := executeGuestTemplate(*provision.Content, instDir, y.User, y.Param); err == nil {
					provision.Content = ptr.Of(out.String())
				} else {
					logrus.WithError(err).Warnf("Couldn't process data content %q as a template", *provision.Content)
				}
			}
			if provision.Overwrite == nil {
				provision.Overwrite = ptr.Of(true)
			}
		}
		if provision.Mode == limatype.ProvisionModeYQ {
			if provision.Expression != nil {
				if out, err := executeGuestTemplate(*provision.Expression, instDir, y.User, y.Param); err == nil {
					provision.Expression = ptr.Of(out.String())
				} else {
					logrus.WithError(err).Warnf("Couldn't process expression %q as a template", *provision.Expression)
				}
			}
			if provision.Format == nil {
				provision.Format = ptr.Of("auto")
			}
		}
		if provision.Mode == limatype.ProvisionModeData || provision.Mode == limatype.ProvisionModeYQ {
			if provision.Owner == nil {
				switch *y.OS {
				case limatype.DARWIN, limatype.FREEBSD:
					provision.Owner = ptr.Of("root:wheel")
				default:
					provision.Owner = ptr.Of("root:root")
				}
			} else {
				if out, err := executeGuestTemplate(*provision.Owner, instDir, y.User, y.Param); err == nil {
					provision.Owner = ptr.Of(out.String())
				} else {
					logrus.WithError(err).Warnf("Couldn't process owner %q as a template", *provision.Owner)
				}
			}
			// Path is required; validation will throw an error when it is nil
			if provision.Path != nil {
				if out, err := executeGuestTemplate(*provision.Path, instDir, y.User, y.Param); err == nil {
					provision.Path = ptr.Of(out.String())
				} else {
					logrus.WithError(err).Warnf("Couldn't process path %q as a template", *provision.Path)
				}
			}
			if provision.Permissions == nil {
				provision.Permissions = ptr.Of("644")
			}
		}
		if provision.Script == nil {
			provision.Script = ptr.Of("")
		}
		if *provision.Script != "" {
			if out, err := executeGuestTemplate(*provision.Script, instDir, y.User, y.Param); err == nil {
				*provision.Script = out.String()
			} else {
				logrus.WithError(err).Warnf("Couldn't process provisioning script %q as a template", *provision.Script)
			}
		}
	}

	if y.GuestInstallPrefix == nil {
		y.GuestInstallPrefix = d.GuestInstallPrefix
	}
	if o.GuestInstallPrefix != nil {
		y.GuestInstallPrefix = o.GuestInstallPrefix
	}
	if y.GuestInstallPrefix == nil {
		y.GuestInstallPrefix = ptr.Of(defaultGuestInstallPrefix())
	}

	if y.UpgradePackages == nil {
		y.UpgradePackages = d.UpgradePackages
	}
	if o.UpgradePackages != nil {
		y.UpgradePackages = o.UpgradePackages
	}
	if y.UpgradePackages == nil {
		y.UpgradePackages = ptr.Of(false)
	}

	if y.Containerd.System == nil {
		y.Containerd.System = d.Containerd.System
	}
	if o.Containerd.System != nil {
		y.Containerd.System = o.Containerd.System
	}
	if y.Containerd.System == nil {
		y.Containerd.System = ptr.Of(false)
	}
	if y.Containerd.User == nil {
		y.Containerd.User = d.Containerd.User
	}
	if o.Containerd.User != nil {
		y.Containerd.User = o.Containerd.User
	}
	if y.Containerd.User == nil {
		switch *y.Arch {
		case limatype.X8664, limatype.AARCH64:
			y.Containerd.User = ptr.Of(true)
		default:
			y.Containerd.User = ptr.Of(false)
		}
	}

	y.Containerd.Archives = slices.Concat(o.Containerd.Archives, y.Containerd.Archives, d.Containerd.Archives)
	if len(y.Containerd.Archives) == 0 {
		y.Containerd.Archives = defaultContainerdArchives()
	}
	for i := range y.Containerd.Archives {
		f := &y.Containerd.Archives[i]
		if f.Arch == "" {
			f.Arch = *y.Arch
		}
	}

	y.Probes = slices.Concat(o.Probes, y.Probes, d.Probes)
	for i := range y.Probes {
		probe := &y.Probes[i]
		if probe.Mode == "" {
			probe.Mode = limatype.ProbeModeReadiness
		}
		if probe.Description == "" {
			probe.Description = fmt.Sprintf("user probe %d/%d", i+1, len(y.Probes))
		}
		if probe.Script == nil {
			probe.Script = ptr.Of("")
		}
		if out, err := executeGuestTemplate(*probe.Script, instDir, y.User, y.Param); err == nil {
			probe.Script = ptr.Of(out.String())
		} else {
			logrus.WithError(err).Warnf("Couldn't process probing script %q as a template", *probe.Script)
		}
	}

	y.PortForwards = slices.Concat(o.PortForwards, y.PortForwards, d.PortForwards)
	for i := range y.PortForwards {
		FillPortForwardDefaults(&y.PortForwards[i], instDir, y.User, y.Param)
		// After defaults processing the singular HostPort and GuestPort values should not be used again.
	}

	y.CopyToHost = slices.Concat(o.CopyToHost, y.CopyToHost, d.CopyToHost)
	for i := range y.CopyToHost {
		FillCopyToHostDefaults(&y.CopyToHost[i], instDir, y.User, y.Param)
	}

	if y.HostResolver.Enabled == nil {
		y.HostResolver.Enabled = d.HostResolver.Enabled
	}
	if o.HostResolver.Enabled != nil {
		y.HostResolver.Enabled = o.HostResolver.Enabled
	}
	if y.HostResolver.Enabled == nil {
		y.HostResolver.Enabled = ptr.Of(true)
	}

	if y.HostResolver.IPv6 == nil {
		y.HostResolver.IPv6 = d.HostResolver.IPv6
	}
	if o.HostResolver.IPv6 != nil {
		y.HostResolver.IPv6 = o.HostResolver.IPv6
	}
	if y.HostResolver.IPv6 == nil {
		y.HostResolver.IPv6 = ptr.Of(false)
	}

	if y.PropagateProxyEnv == nil {
		y.PropagateProxyEnv = d.PropagateProxyEnv
	}
	if o.PropagateProxyEnv != nil {
		y.PropagateProxyEnv = o.PropagateProxyEnv
	}
	if y.PropagateProxyEnv == nil {
		y.PropagateProxyEnv = ptr.Of(true)
	}

	networks := make([]limatype.Network, 0, len(d.Networks)+len(y.Networks)+len(o.Networks))
	iface := make(map[string]int)
	for _, nw := range slices.Concat(d.Networks, y.Networks, o.Networks) {
		if i, ok := iface[nw.Interface]; ok {
			if nw.Socket != "" {
				networks[i].Socket = nw.Socket
				networks[i].Lima = ""
			}
			if nw.Lima != "" {
				if nw.Socket != "" {
					// We can't return an error, so just log it, and prefer `lima` over `socket`
					logrus.Errorf("Network %q has both socket=%q and lima=%q fields; ignoring socket",
						nw.Interface, nw.Socket, nw.Lima)
				}
				networks[i].Lima = nw.Lima
				networks[i].Socket = ""
			}
			if nw.MACAddress != "" {
				networks[i].MACAddress = nw.MACAddress
			}
			if nw.Metric != nil {
				networks[i].Metric = nw.Metric
			}
		} else {
			// unnamed network definitions are not combined/overwritten
			if nw.Interface != "" {
				iface[nw.Interface] = len(networks)
			}
			networks = append(networks, nw)
		}
	}
	y.Networks = networks
	for i := range y.Networks {
		nw := &y.Networks[i]
		if nw.MACAddress == "" {
			// every interface in every limayaml file must get its own unique MAC address
			nw.MACAddress = MACAddress(fmt.Sprintf("%s#%d", filePath, i))
		}
		if nw.Interface == "" {
			nw.Interface = "lima" + strconv.Itoa(i)
		}
		if nw.Metric == nil {
			nw.Metric = ptr.Of(uint32(100))
		}
	}

	y.MountTypesUnsupported = slices.Concat(o.MountTypesUnsupported, y.MountTypesUnsupported, d.MountTypesUnsupported)

	// MountType has to be resolved before resolving Mounts
	if y.MountType == nil {
		y.MountType = d.MountType
	}
	if o.MountType != nil {
		y.MountType = o.MountType
	}

	if y.MountInotify == nil {
		y.MountInotify = d.MountInotify
	}
	if o.MountInotify != nil {
		y.MountInotify = o.MountInotify
	}
	if y.MountInotify == nil {
		y.MountInotify = ptr.Of(false)
	}

	// Combine all mounts; highest priority entry determines writable status.
	// Only works for exact matches; does not normalize case or resolve symlinks.
	mounts := make([]limatype.Mount, 0, len(d.Mounts)+len(y.Mounts)+len(o.Mounts))
	mountPoint := make(map[string]int)
	for _, mount := range slices.Concat(d.Mounts, y.Mounts, o.Mounts) {
		if out, err := executeHostTemplate(mount.Location, instDir, y.Param); err == nil {
			mount.Location = filepath.Clean(out.String())
		} else {
			logrus.WithError(err).Warnf("Couldn't process mount location %q as a template", mount.Location)
		}
		// Expand a path that begins with `~`. Relative paths are not modified, and rejected by Validate() later.
		if localpathutil.IsTildePath(mount.Location) {
			if location, err := localpathutil.Expand(mount.Location); err == nil {
				mount.Location = location
			} else {
				logrus.WithError(err).Warnf("Couldn't expand location %q", mount.Location)
			}
		}
		if mount.MountPoint == nil {
			mountLocation := mount.Location
			if runtime.GOOS == "windows" {
				var err error
				mountLocation, err = ioutilx.WindowsSubsystemPath(ctx, mountLocation)
				if err != nil {
					logrus.WithError(err).Warnf("Couldn't convert location %q into mount target", mount.Location)
				}
			}
			mount.MountPoint = ptr.Of(mountLocation)
		} else {
			if out, err := executeGuestTemplate(*mount.MountPoint, instDir, y.User, y.Param); err == nil {
				mount.MountPoint = ptr.Of(out.String())
			} else {
				logrus.WithError(err).Warnf("Couldn't process mount point %q as a template", *mount.MountPoint)
			}
		}
		if i, ok := mountPoint[*mount.MountPoint]; ok {
			if mount.SSHFS.Cache != nil {
				mounts[i].SSHFS.Cache = mount.SSHFS.Cache
			}
			if mount.SSHFS.FollowSymlinks != nil {
				mounts[i].SSHFS.FollowSymlinks = mount.SSHFS.FollowSymlinks
			}
			if mount.SSHFS.SFTPDriver != nil {
				mounts[i].SSHFS.SFTPDriver = mount.SSHFS.SFTPDriver
			}
			if mount.NineP.SecurityModel != nil {
				mounts[i].NineP.SecurityModel = mount.NineP.SecurityModel
			}
			if mount.NineP.ProtocolVersion != nil {
				mounts[i].NineP.ProtocolVersion = mount.NineP.ProtocolVersion
			}
			if mount.NineP.Msize != nil {
				mounts[i].NineP.Msize = mount.NineP.Msize
			}
			if mount.NineP.Cache != nil {
				mounts[i].NineP.Cache = mount.NineP.Cache
			}
			if mount.Virtiofs.QueueSize != nil {
				mounts[i].Virtiofs.QueueSize = mount.Virtiofs.QueueSize
			}
			if mount.Writable != nil {
				mounts[i].Writable = mount.Writable
			}
			if mount.MountPoint != nil {
				mounts[i].MountPoint = mount.MountPoint
			}
		} else {
			mountPoint[*mount.MountPoint] = len(mounts)
			mounts = append(mounts, mount)
		}
	}
	y.Mounts = mounts

	for i := range y.Mounts {
		mount := &y.Mounts[i]
		if mount.SSHFS.Cache == nil {
			mount.SSHFS.Cache = ptr.Of(true)
		}
		if mount.SSHFS.FollowSymlinks == nil {
			mount.SSHFS.FollowSymlinks = ptr.Of(false)
		}
		if mount.SSHFS.SFTPDriver == nil {
			mount.SSHFS.SFTPDriver = ptr.Of("")
		}
		if mount.NineP.SecurityModel == nil {
			mounts[i].NineP.SecurityModel = ptr.Of(Default9pSecurityModel)
		}
		if mount.NineP.ProtocolVersion == nil {
			mounts[i].NineP.ProtocolVersion = ptr.Of(Default9pProtocolVersion)
		}
		if mount.NineP.Msize == nil {
			mounts[i].NineP.Msize = ptr.Of(Default9pMsize)
		}
		if mount.Writable == nil {
			mount.Writable = ptr.Of(false)
		}
		if mount.NineP.Cache == nil {
			if *mount.Writable {
				mounts[i].NineP.Cache = ptr.Of(Default9pCacheForRW)
			} else {
				mounts[i].NineP.Cache = ptr.Of(Default9pCacheForRO)
			}
		}
	}

	// Note: DNS lists are not combined; highest priority setting is picked
	if len(y.DNS) == 0 {
		y.DNS = d.DNS
	}
	if len(o.DNS) > 0 {
		y.DNS = o.DNS
	}

	env := make(map[string]string)
	maps.Copy(env, d.Env)
	maps.Copy(env, y.Env)
	maps.Copy(env, o.Env)
	y.Env = env

	param := make(map[string]string)
	maps.Copy(param, d.Param)
	maps.Copy(param, y.Param)
	maps.Copy(param, o.Param)
	y.Param = param

	vmOpts := make(limatype.VMOpts)
	maps.Copy(vmOpts, d.VMOpts)
	maps.Copy(vmOpts, y.VMOpts)
	maps.Copy(vmOpts, o.VMOpts)
	y.VMOpts = vmOpts

	if y.CACertificates.RemoveDefaults == nil {
		y.CACertificates.RemoveDefaults = d.CACertificates.RemoveDefaults
	}
	if o.CACertificates.RemoveDefaults != nil {
		y.CACertificates.RemoveDefaults = o.CACertificates.RemoveDefaults
	}
	if y.CACertificates.RemoveDefaults == nil {
		y.CACertificates.RemoveDefaults = ptr.Of(false)
	}

	y.CACertificates.Files = unique(slices.Concat(d.CACertificates.Files, y.CACertificates.Files, o.CACertificates.Files))
	y.CACertificates.Certs = unique(slices.Concat(d.CACertificates.Certs, y.CACertificates.Certs, o.CACertificates.Certs))

	if y.NestedVirtualization == nil {
		y.NestedVirtualization = d.NestedVirtualization
	}
	if o.NestedVirtualization != nil {
		y.NestedVirtualization = o.NestedVirtualization
	}
	if y.NestedVirtualization == nil {
		y.NestedVirtualization = ptr.Of(false)
	}

	if y.Plain == nil {
		y.Plain = d.Plain
	}
	if o.Plain != nil {
		y.Plain = o.Plain
	}
	if y.Plain == nil {
		y.Plain = ptr.Of(false)
	}

	fixUpForPlainMode(y)
}

// ExistingLimaVersion returns empty if the instance was created with Lima prior to v0.20.
func ExistingLimaVersion(instDir string) string {
	if !IsExistingInstanceDir(instDir) {
		return version.Version
	}

	limaVersionFile := filepath.Join(instDir, filenames.LimaVersion)
	if b, err := os.ReadFile(limaVersionFile); err == nil {
		return strings.TrimSpace(string(b))
	} else if !errors.Is(err, os.ErrNotExist) {
		logrus.WithError(err).Warnf("Failed to read %q", limaVersionFile)
	}

	return version.Version
}

func fixUpForPlainMode(y *limatype.LimaYAML) {
	if !*y.Plain {
		return
	}
	deleteNonStaticPortForwards(&y.PortForwards)
	y.Mounts = nil
	y.Containerd.System = ptr.Of(false)
	y.Containerd.User = ptr.Of(false)
	y.TimeZone = ptr.Of("")
}

// deleteNonStaticPortForwards removes all non-static port forwarding rules in case of Plain mode.
func deleteNonStaticPortForwards(portForwards *[]limatype.PortForward) {
	staticPortForwards := make([]limatype.PortForward, 0, len(*portForwards))
	for _, rule := range *portForwards {
		if rule.Static {
			staticPortForwards = append(staticPortForwards, rule)
		}
	}
	*portForwards = staticPortForwards
}

func executeGuestTemplate(format, instDir string, user limatype.User, param map[string]string) (bytes.Buffer, error) {
	tmpl, err := template.New("").Parse(format)
	if err == nil {
		name := filepath.Base(instDir)
		data := map[string]any{
			"Name":     name,
			"Hostname": hostname.FromInstName(name), // TODO: support customization
			"UID":      *user.UID,
			"User":     *user.Name,
			"Home":     *user.Home,
			"Param":    param,
		}
		var out bytes.Buffer
		err = tmpl.Execute(&out, data)
		if err == nil {
			return out, nil
		}
	}
	return bytes.Buffer{}, err
}

func executeHostTemplate(format, instDir string, param map[string]string) (bytes.Buffer, error) {
	tmpl, err := template.New("").Parse(format)
	if err == nil {
		limaHome, _ := dirnames.LimaDir()
		globalTempDir := "/tmp"
		if runtime.GOOS == "windows" {
			// On Windows the global temp directory "%SystemRoot%\Temp" (normally "C:\Windows\Temp")
			// is not writable by regular users.
			globalTempDir = os.TempDir()
		}
		data := map[string]any{
			"Dir":  instDir,
			"Name": filepath.Base(instDir),
			// TODO: add hostname fields for the host and the guest
			"UID":           currentUser.Uid,
			"User":          currentUser.Username,
			"Home":          userHomeDir,
			"Param":         param,
			"GlobalTempDir": globalTempDir,
			"TempDir":       os.TempDir(),

			"Instance": filepath.Base(instDir), // DEPRECATED, use `{{.Name}}`
			"LimaHome": limaHome,               // DEPRECATED, use `{{.Dir}}` instead of `{{.LimaHome}}/{{.Instance}}`
		}
		var out bytes.Buffer
		err = tmpl.Execute(&out, data)
		if err == nil {
			return out, nil
		}
	}
	return bytes.Buffer{}, err
}

func FillPortForwardDefaults(rule *limatype.PortForward, instDir string, user limatype.User, param map[string]string) {
	if rule.Proto == "" {
		rule.Proto = limatype.ProtoAny
	}
	if rule.GuestIP == nil {
		if rule.GuestIPMustBeZero != nil && *rule.GuestIPMustBeZero {
			rule.GuestIP = net.IPv4zero
		} else {
			rule.GuestIP = IPv4loopback1
		}
	}
	if rule.GuestIPMustBeZero == nil {
		rule.GuestIPMustBeZero = ptr.Of(rule.GuestIP.Equal(net.IPv4zero))
	}
	if rule.HostIP == nil {
		rule.HostIP = IPv4loopback1
	}
	if rule.GuestPortRange[0] == 0 && rule.GuestPortRange[1] == 0 {
		if rule.GuestPort == 0 {
			rule.GuestPortRange[0] = 1
			rule.GuestPortRange[1] = 65535
		} else {
			rule.GuestPortRange[0] = rule.GuestPort
			rule.GuestPortRange[1] = rule.GuestPort
		}
	}
	if rule.GuestSocket != "" {
		if out, err := executeGuestTemplate(rule.GuestSocket, instDir, user, param); err == nil {
			rule.GuestSocket = out.String()
		} else {
			logrus.WithError(err).Warnf("Couldn't process guestSocket %q as a template", rule.GuestSocket)
		}
	}
	if rule.HostSocket != "" {
		if out, err := executeHostTemplate(rule.HostSocket, instDir, param); err == nil {
			rule.HostSocket = out.String()
		} else {
			logrus.WithError(err).Warnf("Couldn't process hostSocket %q as a template", rule.HostSocket)
		}
		if !filepath.IsAbs(rule.HostSocket) {
			rule.HostSocket = filepath.Join(instDir, filenames.SocketDir, rule.HostSocket)
		}
	} else if rule.HostPortRange[0] == 0 && rule.HostPortRange[1] == 0 {
		if rule.HostPort == 0 {
			rule.HostPortRange = rule.GuestPortRange
		} else {
			rule.HostPortRange[0] = rule.HostPort
			rule.HostPortRange[1] = rule.HostPort
		}
	}
}

func FillCopyToHostDefaults(rule *limatype.CopyToHost, instDir string, user limatype.User, param map[string]string) {
	if rule.GuestFile != "" {
		if out, err := executeGuestTemplate(rule.GuestFile, instDir, user, param); err == nil {
			rule.GuestFile = out.String()
		} else {
			logrus.WithError(err).Warnf("Couldn't process guest %q as a template", rule.GuestFile)
		}
	}
	if rule.HostFile != "" {
		if out, err := executeHostTemplate(rule.HostFile, instDir, param); err == nil {
			rule.HostFile = out.String()
		} else {
			logrus.WithError(err).Warnf("Couldn't process host %q as a template", rule.HostFile)
		}
	}
}

func IsExistingInstanceDir(dir string) bool {
	// existence of "lima.yaml" does not signify existence of the instance,
	// because the file is created during the initialization of the instance.
	for _, f := range []string{
		filenames.HostAgentStdoutLog, filenames.HostAgentStderrLog,
		filenames.VzIdentifier, filenames.Image, filenames.Disk, filenames.BaseDiskLegacy, filenames.DiffDiskLegacy, filenames.CIDataISO,
	} {
		file := filepath.Join(dir, f)
		if _, err := os.Lstat(file); !errors.Is(err, os.ErrNotExist) {
			return true
		}
	}
	return false
}

func ResolveOS(s *string) limatype.OS {
	if s == nil || *s == "" || *s == "default" {
		return limatype.NewOS("linux")
	}
	return *s
}

func ResolveArch(s *string) limatype.Arch {
	if s == nil || *s == "" || *s == "default" {
		return limatype.NewArch(runtime.GOARCH)
	}
	return *s
}

func IsNativeArch(arch limatype.Arch) bool {
	nativeX8664 := arch == limatype.X8664 && runtime.GOARCH == "amd64"
	nativeAARCH64 := arch == limatype.AARCH64 && runtime.GOARCH == "arm64"
	nativeARMV7L := arch == limatype.ARMV7L && runtime.GOARCH == "arm" && limatype.Goarm() == 7
	nativePPC64LE := arch == limatype.PPC64LE && runtime.GOARCH == "ppc64le"
	nativeRISCV64 := arch == limatype.RISCV64 && runtime.GOARCH == "riscv64"
	nativeS390X := arch == limatype.S390X && runtime.GOARCH == "s390x"
	return nativeX8664 || nativeAARCH64 || nativeARMV7L || nativePPC64LE || nativeRISCV64 || nativeS390X
}

func unique(s []string) []string {
	keys := make(map[string]bool)
	list := []string{}
	for _, entry := range s {
		if _, found := keys[entry]; !found {
			keys[entry] = true
			list = append(list, entry)
		}
	}
	return list
}
