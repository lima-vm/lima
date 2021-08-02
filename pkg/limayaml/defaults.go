package limayaml

import (
	"crypto/sha256"
	"fmt"
	"net"
	"runtime"
	"strconv"

	"github.com/AkihiroSuda/lima/pkg/guestagent/api"
)

func MACAddress(uniqueID string) string {
	// TODO: combine the uniqueID with the host machineID to create a globally unique hash
	sha := sha256.Sum256([]byte(uniqueID))
	// According to https://gitlab.com/wireshark/wireshark/-/blob/master/manuf
	// no well-known MAC addresses start with 0x22.
	hw := append(net.HardwareAddr{0x22}, sha[0:5]...)
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
	if y.SSH.LoadDotSSHPubKeys == nil {
		y.SSH.LoadDotSSHPubKeys = &[]bool{true}[0]
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
	for i := range y.Probes {
		probe := &y.Probes[i]
		if probe.Mode == "" {
			probe.Mode = ProbeModeReadiness
		}
		if probe.Description == "" {
			probe.Description = fmt.Sprintf("user probe %d/%d", i+1, len(y.Probes))
		}
	}
	for i := range y.PortForwards {
		FillPortForwardDefaults(&y.PortForwards[i])
		// After defaults processing the singular HostPort and GuestPort values should not be used again.
	}
	for i := range y.Network.VDE {
		vde := &y.Network.VDE[i]
		if vde.MACAddress == "" {
			// every interface in every limayaml file must get its own unique MAC address
			vde.MACAddress = MACAddress(fmt.Sprintf("%s#%d", filePath, i))
		}
		if vde.Name == "" {
			vde.Name = "vde" + strconv.Itoa(i)
		}
	}
}

func FillPortForwardDefaults(rule *PortForward) {
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
			rule.GuestPortRange[0] = 1024
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
