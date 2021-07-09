package limayaml

import (
	"fmt"
	"runtime"
)

func FillDefault(y *LimaYAML) {
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
	for i := range y.Ports {
		port := &y.Ports[i]
		if port.GuestIP == "" {
			port.GuestIP = "127.0.0.1"
		}
		if port.GuestPortRange[0] == 0 && port.GuestPortRange[1] == 0 {
			port.GuestPortRange[0] = port.GuestPort
			port.GuestPortRange[1] = port.GuestPort
		}
		if port.HostIP == "" {
			port.HostIP = "127.0.0.1"
		}
		if port.HostPortRange[0] == 0 && port.HostPortRange[1] == 0 {
			if port.HostPort == 0 {
				port.HostPort = port.GuestPortRange[0]
			}
			port.HostPortRange[0] = port.HostPort
			port.HostPortRange[1] = port.HostPort
		}
		if port.Proto == "" {
			port.Proto = TCP
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
