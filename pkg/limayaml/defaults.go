package limayaml

import (
	"crypto/sha256"
	"fmt"
	"net"
	"os"
	osuser "os/user"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/lima-vm/lima/pkg/guestagent/api"
	"github.com/lima-vm/lima/pkg/osutil"
	"github.com/lima-vm/lima/pkg/store/dirnames"
	"github.com/lima-vm/lima/pkg/store/filenames"
)

func defaultContainerdArchives() []File {
	const nerdctlVersion = "0.12.1"
	location := func(goarch string) string {
		return "https://github.com/containerd/nerdctl/releases/download/v" + nerdctlVersion + "/nerdctl-full-" + nerdctlVersion + "-linux-" + goarch + ".tar.gz"
	}
	return []File{
		{
			Location: location("amd64"),
			Arch:     X8664,
			Digest:   "sha256:fe9e7e63554795fc4b984c178609a2a9dffa6d69925356f919b8fa30a2f36210",
		},
		{
			Location: location("arm64"),
			Arch:     AARCH64,
			Digest:   "sha256:47fb7f904cd541c5761ae9f4fd385bfa93fa4b36a963a5a54f7c8df647f5d6fc",
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
		user, _ := osutil.LimaUser(false)
		home := fmt.Sprintf("/home/%s.linux", user.Username)
		rule.GuestSocket = strings.Replace(rule.GuestSocket, "${HOME}", home, -1)
		rule.GuestSocket = strings.Replace(rule.GuestSocket, "${UID}", user.Uid, -1)
		rule.GuestSocket = strings.Replace(rule.GuestSocket, "${USER}", user.Username, -1)
	}
	if rule.HostSocket != "" {
		user, _ := osuser.Current()
		home, _ := os.UserHomeDir()
		limaHome, _ := dirnames.LimaDir()
		rule.HostSocket = strings.Replace(rule.HostSocket, "${LIMA_HOME}", limaHome, -1)
		rule.HostSocket = strings.Replace(rule.HostSocket, "${HOME}", home, -1)
		rule.HostSocket = strings.Replace(rule.HostSocket, "${UID}", user.Uid, -1)
		rule.HostSocket = strings.Replace(rule.HostSocket, "${USER}", user.Username, -1)
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
