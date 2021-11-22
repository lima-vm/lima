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

func FillDefault(y *LimaYAML, filePath string) {
	y.Arch = resolveArch(y.Arch)
	for i := range y.Images {
		img := &y.Images[i]
		if img.Arch == "" {
			img.Arch = y.Arch
		}
	}
	if y.CPUs == 0 {
		y.CPUs = 4
	}
	if y.Memory == "" {
		y.Memory = "4GiB"
	}
	if y.Disk == "" {
		y.Disk = "100GiB"
	}
	if y.Video.Display == "" {
		y.Video.Display = "none"
	}
	// y.SSH.LocalPort is not filled here (filled by the hostagent)
	if y.SSH.LoadDotSSHPubKeys == nil {
		y.SSH.LoadDotSSHPubKeys = &[]bool{true}[0]
	}
	if y.SSH.ForwardAgent == nil {
		y.SSH.ForwardAgent = &[]bool{false}[0]
	}
	for i := range y.Provision {
		provision := &y.Provision[i]
		if provision.Mode == "" {
			provision.Mode = ProvisionModeSystem
		}
	}
	if y.Containerd.System == nil {
		y.Containerd.System = &[]bool{false}[0]
	}
	if y.Containerd.User == nil {
		y.Containerd.User = &[]bool{true}[0]
	}
	if len(y.Containerd.Archives) == 0 {
		y.Containerd.Archives = defaultContainerdArchives()
	}
	for i := range y.Containerd.Archives {
		f := &y.Containerd.Archives[i]
		if f.Arch == "" {
			f.Arch = y.Arch
		}
	}
	for i := range y.Probes {
		probe := &y.Probes[i]
		if probe.Mode == "" {
			probe.Mode = ProbeModeReadiness
		}
		if probe.Description == "" {
			probe.Description = fmt.Sprintf("user probe %d/%d", i+1, len(y.Probes))
		}
	}
	instDir := filepath.Dir(filePath)
	for i := range y.PortForwards {
		FillPortForwardDefaults(&y.PortForwards[i], instDir)
		// After defaults processing the singular HostPort and GuestPort values should not be used again.
	}
	if y.UseHostResolver == nil {
		y.UseHostResolver = &[]bool{true}[0]
	}
	if y.PropagateProxyEnv == nil {
		y.PropagateProxyEnv = &[]bool{true}[0]
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
				"Home":     home,
				"Instance": filepath.Base(instDir),
				"LimaHome": limaHome,
				"UID":      user.Uid,
				"User":     user.Username,
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

func resolveArch(s string) Arch {
	if s == "" || s == "default" {
		if runtime.GOARCH == "amd64" {
			return X8664
		} else {
			return AARCH64
		}
	}
	return s
}
