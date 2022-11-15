package limayaml

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"text/template"

	"github.com/lima-vm/lima/pkg/guestagent/api"
	"github.com/lima-vm/lima/pkg/osutil"
	"github.com/lima-vm/lima/pkg/store/dirnames"
	"github.com/lima-vm/lima/pkg/store/filenames"
	"github.com/sirupsen/logrus"
	"github.com/xorcare/pointer"
)

const (
	// Default9pSecurityModel is "none" for supporting symlinks
	// https://gitlab.com/qemu-project/qemu/-/issues/173
	Default9pSecurityModel   string = "none"
	Default9pProtocolVersion string = "9p2000.L"
	Default9pMsize           string = "128KiB"
	Default9pCacheForRO      string = "fscache"
	Default9pCacheForRW      string = "mmap"
)

func defaultContainerdArchives() []File {
	const nerdctlVersion = "1.0.0"
	location := func(goarch string) string {
		return "https://github.com/containerd/nerdctl/releases/download/v" + nerdctlVersion + "/nerdctl-full-" + nerdctlVersion + "-linux-" + goarch + ".tar.gz"
	}
	return []File{
		{
			Location: location("amd64"),
			Arch:     X8664,
			Digest:   "sha256:b7f76a3bf1b8161eb94ebe885945feb2887dfc0d274f9da908a17bc0ef853eb9",
		},
		{
			Location: location("arm64"),
			Arch:     AARCH64,
			Digest:   "sha256:e4c9b9434c88847f4d18f8b84c0d073555e4c949546cfa8a21e719de67afdf70",
		},
		// No riscv64
	}
}

func MACAddress(uniqueID string) string {
	sha := sha256.Sum256([]byte(osutil.MachineID() + uniqueID))
	// "5" is the magic number in the Lima ecosystem.
	// (Visit https://en.wiktionary.org/wiki/lima and Command-F "five")
	//
	// But the second hex number is changed to 2 to satisfy the convention for
	// local MAC addresses (https://en.wikipedia.org/wiki/MAC_address#Ranges_of_group_and_locally_administered_addresses)
	//
	// See also https://gitlab.com/wireshark/wireshark/-/blob/master/manuf to confirm the uniqueness of this prefix.
	hw := append(net.HardwareAddr{0x52, 0x55, 0x55}, sha[0:3]...)
	return hw.String()
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
func FillDefault(y, d, o *LimaYAML, filePath string) {
	if y.VMType == nil {
		y.VMType = d.VMType
	}
	if o.VMType != nil {
		y.VMType = o.VMType
	}
	y.VMType = pointer.String(ResolveVMType(y.VMType))
	if y.Arch == nil {
		y.Arch = d.Arch
	}
	if o.Arch != nil {
		y.Arch = o.Arch
	}
	y.Arch = pointer.String(ResolveArch(y.Arch))

	y.Images = append(append(o.Images, y.Images...), d.Images...)
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

	cpuType := map[Arch]string{
		AARCH64: "cortex-a72",
		// Since https://github.com/lima-vm/lima/pull/494, we use qemu64 cpu for better emulation of x86_64.
		X8664:   "qemu64",
		RISCV64: "rv64", // FIXME: what is the right choice for riscv64?
	}
	for arch := range cpuType {
		if IsNativeArch(arch) && IsAccelOS() {
			if HasHostCPU() {
				cpuType[arch] = "host"
			} else if HasMaxCPU() {
				cpuType[arch] = "max"
			}
		}
	}
	for k, v := range d.CPUType {
		if len(v) > 0 {
			cpuType[k] = v
		}
	}
	for k, v := range y.CPUType {
		if len(v) > 0 {
			cpuType[k] = v
		}
	}
	for k, v := range o.CPUType {
		if len(v) > 0 {
			cpuType[k] = v
		}
	}
	y.CPUType = cpuType

	if y.CPUs == nil {
		y.CPUs = d.CPUs
	}
	if o.CPUs != nil {
		y.CPUs = o.CPUs
	}
	if y.CPUs == nil || *y.CPUs == 0 {
		y.CPUs = pointer.Int(4)
	}

	if y.Memory == nil {
		y.Memory = d.Memory
	}
	if o.Memory != nil {
		y.Memory = o.Memory
	}
	if y.Memory == nil || *y.Memory == "" {
		y.Memory = pointer.String("4GiB")
	}

	if y.Disk == nil {
		y.Disk = d.Disk
	}
	if o.Disk != nil {
		y.Disk = o.Disk
	}
	if y.Disk == nil || *y.Disk == "" {
		y.Disk = pointer.String("100GiB")
	}

	y.AdditionalDisks = append(append(o.AdditionalDisks, y.AdditionalDisks...), d.AdditionalDisks...)

	if y.Video.Display == nil {
		y.Video.Display = d.Video.Display
	}
	if o.Video.Display != nil {
		y.Video.Display = o.Video.Display
	}
	if y.Video.Display == nil || *y.Video.Display == "" {
		y.Video.Display = pointer.String("none")
	}

	if y.Firmware.LegacyBIOS == nil {
		y.Firmware.LegacyBIOS = d.Firmware.LegacyBIOS
	}
	if o.Firmware.LegacyBIOS != nil {
		y.Firmware.LegacyBIOS = o.Firmware.LegacyBIOS
	}
	if y.Firmware.LegacyBIOS == nil {
		y.Firmware.LegacyBIOS = pointer.Bool(false)
	}

	if y.SSH.LocalPort == nil {
		y.SSH.LocalPort = d.SSH.LocalPort
	}
	if o.SSH.LocalPort != nil {
		y.SSH.LocalPort = o.SSH.LocalPort
	}
	if y.SSH.LocalPort == nil {
		// y.SSH.LocalPort value is not filled here (filled by the hostagent)
		y.SSH.LocalPort = pointer.Int(0)
	}
	if y.SSH.LoadDotSSHPubKeys == nil {
		y.SSH.LoadDotSSHPubKeys = d.SSH.LoadDotSSHPubKeys
	}
	if o.SSH.LoadDotSSHPubKeys != nil {
		y.SSH.LoadDotSSHPubKeys = o.SSH.LoadDotSSHPubKeys
	}
	if y.SSH.LoadDotSSHPubKeys == nil {
		y.SSH.LoadDotSSHPubKeys = pointer.Bool(true)
	}

	if y.SSH.ForwardAgent == nil {
		y.SSH.ForwardAgent = d.SSH.ForwardAgent
	}
	if o.SSH.ForwardAgent != nil {
		y.SSH.ForwardAgent = o.SSH.ForwardAgent
	}
	if y.SSH.ForwardAgent == nil {
		y.SSH.ForwardAgent = pointer.Bool(false)
	}

	if y.SSH.ForwardX11 == nil {
		y.SSH.ForwardX11 = d.SSH.ForwardX11
	}
	if o.SSH.ForwardX11 != nil {
		y.SSH.ForwardX11 = o.SSH.ForwardX11
	}
	if y.SSH.ForwardX11 == nil {
		y.SSH.ForwardX11 = pointer.Bool(false)
	}

	if y.SSH.ForwardX11Trusted == nil {
		y.SSH.ForwardX11Trusted = d.SSH.ForwardX11Trusted
	}
	if o.SSH.ForwardX11Trusted != nil {
		y.SSH.ForwardX11Trusted = o.SSH.ForwardX11Trusted
	}
	if y.SSH.ForwardX11Trusted == nil {
		y.SSH.ForwardX11Trusted = pointer.Bool(false)
	}

	hosts := make(map[string]string)
	// Values can be either names or IP addresses. Name values are canonicalized in the hostResolver.
	for k, v := range d.HostResolver.Hosts {
		hosts[k] = v
	}
	for k, v := range y.HostResolver.Hosts {
		hosts[k] = v
	}
	for k, v := range o.HostResolver.Hosts {
		hosts[k] = v
	}
	y.HostResolver.Hosts = hosts

	y.Provision = append(append(o.Provision, y.Provision...), d.Provision...)
	for i := range y.Provision {
		provision := &y.Provision[i]
		if provision.Mode == "" {
			provision.Mode = ProvisionModeSystem
		}
	}

	if y.Containerd.System == nil {
		y.Containerd.System = d.Containerd.System
	}
	if o.Containerd.System != nil {
		y.Containerd.System = o.Containerd.System
	}
	if y.Containerd.System == nil {
		y.Containerd.System = pointer.Bool(false)
	}
	if y.Containerd.User == nil {
		y.Containerd.User = d.Containerd.User
	}
	if o.Containerd.User != nil {
		y.Containerd.User = o.Containerd.User
	}
	if y.Containerd.User == nil {
		y.Containerd.User = pointer.Bool(true)
	}

	y.Containerd.Archives = append(append(o.Containerd.Archives, y.Containerd.Archives...), d.Containerd.Archives...)
	if len(y.Containerd.Archives) == 0 {
		y.Containerd.Archives = defaultContainerdArchives()
	}
	for i := range y.Containerd.Archives {
		f := &y.Containerd.Archives[i]
		if f.Arch == "" {
			f.Arch = *y.Arch
		}
	}

	y.Probes = append(append(o.Probes, y.Probes...), d.Probes...)
	for i := range y.Probes {
		probe := &y.Probes[i]
		if probe.Mode == "" {
			probe.Mode = ProbeModeReadiness
		}
		if probe.Description == "" {
			probe.Description = fmt.Sprintf("user probe %d/%d", i+1, len(y.Probes))
		}
	}

	y.PortForwards = append(append(o.PortForwards, y.PortForwards...), d.PortForwards...)
	instDir := filepath.Dir(filePath)
	for i := range y.PortForwards {
		FillPortForwardDefaults(&y.PortForwards[i], instDir)
		// After defaults processing the singular HostPort and GuestPort values should not be used again.
	}

	// If both `useHostResolved` and `HostResolver.Enabled` are defined in the same config,
	// then the deprecated `useHostResolved` setting is silently ignored.
	if y.HostResolver.Enabled == nil {
		y.HostResolver.Enabled = y.UseHostResolver
	}
	if d.HostResolver.Enabled == nil {
		d.HostResolver.Enabled = d.UseHostResolver
	}
	if o.HostResolver.Enabled == nil {
		o.HostResolver.Enabled = o.UseHostResolver
	}

	if y.HostResolver.Enabled == nil {
		y.HostResolver.Enabled = d.HostResolver.Enabled
	}
	if o.HostResolver.Enabled != nil {
		y.HostResolver.Enabled = o.HostResolver.Enabled
	}
	if y.HostResolver.Enabled == nil {
		y.HostResolver.Enabled = pointer.Bool(true)
	}

	if y.HostResolver.IPv6 == nil {
		y.HostResolver.IPv6 = d.HostResolver.IPv6
	}
	if o.HostResolver.IPv6 != nil {
		y.HostResolver.IPv6 = o.HostResolver.IPv6
	}
	if y.HostResolver.IPv6 == nil {
		y.HostResolver.IPv6 = pointer.Bool(false)
	}

	if y.PropagateProxyEnv == nil {
		y.PropagateProxyEnv = d.PropagateProxyEnv
	}
	if o.PropagateProxyEnv != nil {
		y.PropagateProxyEnv = o.PropagateProxyEnv
	}
	if y.PropagateProxyEnv == nil {
		y.PropagateProxyEnv = pointer.Bool(true)
	}

	if len(y.Network.VDEDeprecated) > 0 && len(y.Networks) == 0 {
		for _, vde := range y.Network.VDEDeprecated {
			network := Network{
				Interface:            vde.Name,
				MACAddress:           vde.MACAddress,
				SwitchPortDeprecated: vde.SwitchPort,
				VNLDeprecated:        vde.VNL,
			}
			y.Networks = append(y.Networks, network)
		}
		y.Network.migrated = true
	}

	networks := make([]Network, 0, len(d.Networks)+len(y.Networks)+len(o.Networks))
	iface := make(map[string]int)
	for _, nw := range append(append(d.Networks, y.Networks...), o.Networks...) {
		if i, ok := iface[nw.Interface]; ok {
			if nw.VNLDeprecated != "" {
				networks[i].VNLDeprecated = nw.VNLDeprecated
				networks[i].SwitchPortDeprecated = nw.SwitchPortDeprecated
				networks[i].Socket = ""
				networks[i].Lima = ""
			}
			if nw.Socket != "" {
				if nw.VNLDeprecated != "" {
					// We can't return an error, so just log it, and prefer `socket` over `vnl`
					logrus.Errorf("Network %q has both vnl=%q and socket=%q fields; ignoring vnl",
						nw.Interface, nw.VNLDeprecated, nw.Socket)
				}
				networks[i].Socket = nw.Socket
				networks[i].VNLDeprecated = ""
				networks[i].SwitchPortDeprecated = 0
				networks[i].Lima = ""
			}
			if nw.Lima != "" {
				if nw.VNLDeprecated != "" {
					// We can't return an error, so just log it, and prefer `lima` over `vnl`
					logrus.Errorf("Network %q has both vnl=%q and lima=%q fields; ignoring vnl",
						nw.Interface, nw.VNLDeprecated, nw.Lima)
				}
				if nw.Socket != "" {
					// We can't return an error, so just log it, and prefer `lima` over `socket`
					logrus.Errorf("Network %q has both socket=%q and lima=%q fields; ignoring socket",
						nw.Interface, nw.Socket, nw.Lima)
				}
				networks[i].Lima = nw.Lima
				networks[i].Socket = ""
				networks[i].VNLDeprecated = ""
				networks[i].SwitchPortDeprecated = 0
			}
			if nw.MACAddress != "" {
				networks[i].MACAddress = nw.MACAddress
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
	}

	// Combine all mounts; highest priority entry determines writable status.
	// Only works for exact matches; does not normalize case or resolve symlinks.
	mounts := make([]Mount, 0, len(d.Mounts)+len(y.Mounts)+len(o.Mounts))
	location := make(map[string]int)
	for _, mount := range append(append(d.Mounts, y.Mounts...), o.Mounts...) {
		if i, ok := location[mount.Location]; ok {
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
			if mount.Writable != nil {
				mounts[i].Writable = mount.Writable
			}
			if mount.MountPoint != "" {
				mounts[i].MountPoint = mount.MountPoint
			}
		} else {
			location[mount.Location] = len(mounts)
			mounts = append(mounts, mount)
		}
	}
	y.Mounts = mounts

	for i := range y.Mounts {
		mount := &y.Mounts[i]
		if mount.SSHFS.Cache == nil {
			mount.SSHFS.Cache = pointer.Bool(true)
		}
		if mount.SSHFS.FollowSymlinks == nil {
			mount.SSHFS.FollowSymlinks = pointer.Bool(false)
		}
		if mount.SSHFS.SFTPDriver == nil {
			mount.SSHFS.SFTPDriver = pointer.String("")
		}
		if mount.NineP.SecurityModel == nil {
			mounts[i].NineP.SecurityModel = pointer.String(Default9pSecurityModel)
		}
		if mount.NineP.ProtocolVersion == nil {
			mounts[i].NineP.ProtocolVersion = pointer.String(Default9pProtocolVersion)
		}
		if mount.NineP.Msize == nil {
			mounts[i].NineP.Msize = pointer.String(Default9pMsize)
		}
		if mount.Writable == nil {
			mount.Writable = pointer.Bool(false)
		}
		if mount.NineP.Cache == nil {
			if *mount.Writable {
				mounts[i].NineP.Cache = pointer.String(Default9pCacheForRW)
			} else {
				mounts[i].NineP.Cache = pointer.String(Default9pCacheForRO)
			}
		}
		if mount.MountPoint == "" {
			mounts[i].MountPoint = mount.Location
		}
	}

	if y.MountType == nil {
		y.MountType = d.MountType
	}
	if o.MountType != nil {
		y.MountType = o.MountType
	}
	if y.MountType == nil || *y.MountType == "" {
		y.MountType = pointer.String(REVSSHFS)
	}

	// Note: DNS lists are not combined; highest priority setting is picked
	if len(y.DNS) == 0 {
		y.DNS = d.DNS
	}
	if len(o.DNS) > 0 {
		y.DNS = o.DNS
	}

	env := make(map[string]string)
	for k, v := range d.Env {
		env[k] = v
	}
	for k, v := range y.Env {
		env[k] = v
	}
	for k, v := range o.Env {
		env[k] = v
	}
	y.Env = env

	if y.CACertificates.RemoveDefaults == nil {
		y.CACertificates.RemoveDefaults = d.CACertificates.RemoveDefaults
	}
	if o.CACertificates.RemoveDefaults != nil {
		y.CACertificates.RemoveDefaults = o.CACertificates.RemoveDefaults
	}
	if y.CACertificates.RemoveDefaults == nil {
		y.CACertificates.RemoveDefaults = pointer.Bool(false)
	}

	caFiles := unique(append(append(d.CACertificates.Files, y.CACertificates.Files...), o.CACertificates.Files...))
	y.CACertificates.Files = caFiles

	caCerts := unique(append(append(d.CACertificates.Certs, y.CACertificates.Certs...), o.CACertificates.Certs...))
	y.CACertificates.Certs = caCerts
}

func FillPortForwardDefaults(rule *PortForward, instDir string) {
	if rule.Proto == "" {
		rule.Proto = TCP
	}
	if rule.GuestIP == nil {
		if rule.GuestIPMustBeZero {
			rule.GuestIP = net.IPv4zero
		} else {
			rule.GuestIP = api.IPv4loopback1
		}
	}
	if rule.HostIP == nil {
		rule.HostIP = api.IPv4loopback1
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
	if rule.HostPortRange[0] == 0 && rule.HostPortRange[1] == 0 {
		if rule.HostPort == 0 {
			rule.HostPortRange = rule.GuestPortRange
		} else {
			rule.HostPortRange[0] = rule.HostPort
			rule.HostPortRange[1] = rule.HostPort
		}
	}
	if rule.GuestSocket != "" {
		tmpl, err := template.New("").Parse(rule.GuestSocket)
		if err == nil {
			user, _ := osutil.LimaUser(false)
			data := map[string]string{
				"Home": fmt.Sprintf("/home/%s.linux", user.Username),
				"UID":  user.Uid,
				"User": user.Username,
			}
			var out bytes.Buffer
			if err := tmpl.Execute(&out, data); err == nil {
				rule.GuestSocket = out.String()
			} else {
				logrus.WithError(err).Warnf("Couldn't process guestSocket %q as a template", rule.GuestSocket)
			}
		}
	}
	if rule.HostSocket != "" {
		tmpl, err := template.New("").Parse(rule.HostSocket)
		if err == nil {
			user, _ := osutil.LimaUser(false)
			home, _ := os.UserHomeDir()
			limaHome, _ := dirnames.LimaDir()
			data := map[string]string{
				"Dir":  instDir,
				"Home": home,
				"Name": filepath.Base(instDir),
				"UID":  user.Uid,
				"User": user.Username,

				"Instance": filepath.Base(instDir), // DEPRECATED, use `{{.Name}}`
				"LimaHome": limaHome,               // DEPRECATED, (use `Dir` instead of `{{.LimaHome}}/{{.Instance}}`
			}
			var out bytes.Buffer
			if err := tmpl.Execute(&out, data); err == nil {
				rule.HostSocket = out.String()
			} else {
				logrus.WithError(err).Warnf("Couldn't process hostSocket %q as a template", rule.HostSocket)
			}
		}
		if !filepath.IsAbs(rule.HostSocket) {
			rule.HostSocket = filepath.Join(instDir, filenames.SocketDir, rule.HostSocket)
		}
	}
}

func NewArch(arch string) Arch {
	switch arch {
	case "amd64":
		return X8664
	case "arm64":
		return AARCH64
	case "riscv64":
		return RISCV64
	default:
		logrus.Warnf("Unknown arch: %s", arch)
		return arch
	}
}

func NewVMType(driver string) VMType {
	switch driver {
	case "vz":
		return VZ
	case "qemu":
		return QEMU
	default:
		logrus.Warnf("Unknown driver: %s", driver)
		return driver
	}
}

func ResolveVMType(s *string) VMType {
	if s == nil || *s == "" || *s == "default" {
		return QEMU
	}
	return NewVMType(*s)
}

func ResolveArch(s *string) Arch {
	if s == nil || *s == "" || *s == "default" {
		return NewArch(runtime.GOARCH)
	}
	return *s
}

func IsAccelOS() bool {
	switch runtime.GOOS {
	case "darwin", "linux", "netbsd", "windows":
		// Accelerator
		return true
	}
	// Using TCG
	return false
}

func HasHostCPU() bool {
	switch runtime.GOOS {
	case "darwin", "linux":
		return true
	case "netbsd", "windows":
		return false
	}
	// Not reached
	return false
}

func HasMaxCPU() bool {
	// WHPX: Unexpected VP exit code 4
	return runtime.GOOS != "windows"
}

func IsNativeArch(arch Arch) bool {
	nativeX8664 := arch == X8664 && runtime.GOARCH == "amd64"
	nativeAARCH64 := arch == AARCH64 && runtime.GOARCH == "arm64"
	nativeRISCV64 := arch == RISCV64 && runtime.GOARCH == "riscv64"
	return nativeX8664 || nativeAARCH64 || nativeRISCV64
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
