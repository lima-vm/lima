package networks

import (
	"fmt"
	"github.com/lima-vm/lima/pkg/store"
)

const (
	Switch = "switch"
	VMNet  = "vmnet"
)

// Commands in `sudoers` cannot use quotes, so all arguments are printed via "%s"
// and not "%q". config.Paths.* entries must not include any whitespace!

func (config *NetworksConfig) Check(name string) error {
	if _, ok := config.Networks[name]; ok {
		return nil
	}
	return fmt.Errorf("network %q is not defined", name)
}

func (config *NetworksConfig) VDESock(name string) string {
	return fmt.Sprintf("%s/%s.ctl", config.Paths.VarRun, name)
}

func (config *NetworksConfig) PIDFile(name, daemon string) string {
	return fmt.Sprintf("%s/%s_%s.pid", config.Paths.VarRun, name, daemon)
}

func (config *NetworksConfig) LogFile(name, daemon, stream string) string {
	networksDir, _ := store.LimaNetworksDir()
	return fmt.Sprintf("%s/%s_%s.%s.log", networksDir, name, daemon, stream)
}

func (config *NetworksConfig) DaemonUser(daemon string) string {
	switch daemon {
	case Switch:
		return "daemon"
	case VMNet:
		return "root"
	}
	panic("daemonuser")
}

func (config *NetworksConfig) DaemonGroup(daemon string) string {
	switch daemon {
	case Switch:
		return config.Group
	case VMNet:
		return "wheel"
	}
	panic("daemongroup")
}

func (config *NetworksConfig) MkdirCmd() string {
	return fmt.Sprintf("/bin/mkdir -m 775 -p %s", config.Paths.VarRun)
}

func (config *NetworksConfig) StartCmd(name, daemon string) string {
	var cmd string
	switch daemon {
	case Switch:
		cmd = fmt.Sprintf("%s --pidfile=%s --sock=%s --group=%s --dirmode=0770 --nostdin",
			config.Paths.VDESwitch, config.PIDFile(name, Switch), config.VDESock(name), config.Group)
	case VMNet:
		nw := config.Networks[name]
		cmd = fmt.Sprintf("%s --pidfile=%s --vde-group=%s --vmnet-mode=%s",
			config.Paths.VDEVMNet, config.PIDFile(name, VMNet), config.Group, nw.Mode)
		switch nw.Mode {
		case ModeBridged:
			cmd += fmt.Sprintf(" --vmnet-interface=%s", nw.Interface)
		case ModeHost, ModeShared:
			cmd += fmt.Sprintf(" --vmnet-gateway=%s --vmnet-dhcp-end=%s --vmnet-mask=%s",
				nw.Gateway, nw.DHCPEnd, nw.NetMask)
		}
		cmd += " " + config.VDESock(name)
	}
	return cmd
}

func (config *NetworksConfig) StopCmd(name, daemon string) string {
	return fmt.Sprintf("/usr/bin/pkill -F %s", config.PIDFile(name, daemon))
}
