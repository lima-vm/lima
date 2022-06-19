package networks

import (
	"fmt"
	"path/filepath"

	"github.com/lima-vm/lima/pkg/osutil"
	"github.com/lima-vm/lima/pkg/store/dirnames"
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
	return filepath.Join(config.Paths.VarRun, fmt.Sprintf("%s.ctl", name))
}

func (config *NetworksConfig) PIDFile(name, daemon string) string {
	return filepath.Join(config.Paths.VarRun, fmt.Sprintf("%s_%s.pid", name, daemon))
}

func (config *NetworksConfig) LogFile(name, daemon, stream string) string {
	networksDir, _ := dirnames.LimaNetworksDir()
	return filepath.Join(networksDir, fmt.Sprintf("%s_%s.%s.log", name, daemon, stream))
}

func (config *NetworksConfig) User(daemon string) (osutil.User, error) {
	switch daemon {
	case Switch:
		user, err := osutil.LookupUser("daemon")
		if err != nil {
			return user, err
		}
		group, err := osutil.LookupGroup(config.Group)
		user.Group = group.Name
		user.Gid = group.Gid
		return user, err
	case VMNet:
		return osutil.LookupUser("root")
	}
	return osutil.User{}, fmt.Errorf("daemon %q not defined", daemon)
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
