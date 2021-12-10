package limayaml

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"net"
	"os"
	osuser "os/user"
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

func defaultContainerdArchives() []File {
	const nerdctlVersion = "0.14.0"
	location := func(goarch string) string {
		return "https://github.com/containerd/nerdctl/releases/download/v" + nerdctlVersion + "/nerdctl-full-" + nerdctlVersion + "-linux-" + goarch + ".tar.gz"
	}
	return []File{
		{
			Location: location("amd64"),
			Arch:     X8664,
			Digest:   "sha256:3423cb589bb5058ff9ed55f6823adec1299fe2e576612fc6f706fe07eddd398b",
		},
		{
			Location: location("arm64"),
			Arch:     AARCH64,
			Digest:   "sha256:32898576fa89392d1af8c21ff3854c0f54d2c66c0de87598be813f25051366e5",
		},
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
// - Mounts are appended in d, y, o order, but "merged" when the Location matches a previous entry;
//   the highest priority Writable setting wins.
// - DNS are picked from the highest priority where DNS is not empty.
func FillDefault(y, d, o *LimaYAML, filePath string) {
	if y.Arch == nil {
		y.Arch = d.Arch
	}
	if o.Arch != nil {
		y.Arch = o.Arch
	}
	y.Arch = pointer.String(resolveArch(y.Arch))

	y.Images = append(append(o.Images, y.Images...), d.Images...)
	for i := range y.Images {
		img := &y.Images[i]
		if img.Arch == "" {
			img.Arch = *y.Arch
		}
	}

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

	if y.UseHostResolver == nil {
		y.UseHostResolver = d.UseHostResolver
	}
	if o.UseHostResolver != nil {
		y.UseHostResolver = o.UseHostResolver
	}
	if y.UseHostResolver == nil {
		y.UseHostResolver = pointer.Bool(true)
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
				Interface:  vde.Name,
				MACAddress: vde.MACAddress,
				SwitchPort: vde.SwitchPort,
				VNL:        vde.VNL,
			}
			y.Networks = append(y.Networks, network)
		}
		y.Network.migrated = true
	}

	networks := make([]Network, 0, len(d.Networks)+len(y.Networks)+len(o.Networks))
	iface := make(map[string]int)
	for _, nw := range append(append(d.Networks, y.Networks...), o.Networks...) {
		if i, ok := iface[nw.Interface]; ok {
			if nw.VNL != "" {
				networks[i].VNL = nw.VNL
				networks[i].SwitchPort = nw.SwitchPort
				networks[i].Lima = ""
			}
			if nw.Lima != "" {
				if nw.VNL != "" {
					// We can't return an error, so just log it, and prefer `lima` over `vnl`
					logrus.Errorf("Network %q has both vnl=%q and lima=%q fields; ignoring vnl",
						nw.Interface, nw.VNL, nw.Lima)
				}
				networks[i].Lima = nw.Lima
				networks[i].VNL = ""
				networks[i].SwitchPort = 0
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
			mounts[i].Writable = mount.Writable
		} else {
			location[mount.Location] = len(mounts)
			mounts = append(mounts, mount)
		}
	}
	y.Mounts = mounts

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
}

func FillPortForwardDefaults(rule *PortForward, instDir string) {
	if rule.Proto == "" {
		rule.Proto = TCP
	}
	if rule.GuestIP == nil {
		rule.GuestIP = api.IPv4loopback1
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
			user, _ := osuser.Current()
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

func resolveArch(s *string) Arch {
	if s == nil || *s == "" || *s == "default" {
		if runtime.GOARCH == "amd64" {
			return X8664
		} else {
			return AARCH64
		}
	}
	return *s
}
